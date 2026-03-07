//! Contester service — RPC method implementations.
//!
//! Handles: Identify, LocalExecute, LocalExecuteConnected, Put, Get, Stat, Clear, GridfsCopy.

mod sandbox;

use std::path::Path;
use std::sync::Arc;

use anyhow::Result;
use prost::Message;
use serde::Deserialize;

use contester_proto::*;
use contester_rpc4::{RpcRequest, ServerCodec};
use contester_subprocess::{
    ExecutionFlags, Redirect, RedirectMode, Subprocess, SubprocessResult,
    du_from_micros, get_micros,
};
#[cfg(windows)]
use contester_subprocess::WindowsLoginSession;

use sandbox::{Sandbox, SandboxPair, get_sandbox_by_id, get_sandbox_by_path, resolve_path};

/// Convert a tools::FileStat to a proto::FileStat.
fn tools_stat_to_proto(s: &contester_tools::FileStat) -> contester_proto::FileStat {
    contester_proto::FileStat {
        name: s.name.clone(),
        is_directory: s.is_directory,
        size: s.size,
        checksum: s.checksum.clone().unwrap_or_default(),
    }
}

// ── Config ──────────────────────────────────────────────────────────────────

#[derive(Deserialize)]
pub struct ContesterConfig {
    pub server: String,
    #[serde(default)]
    pub passwords: String,
    pub path: String,
    #[serde(default)]
    pub sandbox_count: usize,
}

// ── Contester service ───────────────────────────────────────────────────────

pub struct Contester {
    pub invoker_id: String,
    pub sandboxes: Vec<SandboxPair>,
    pub env: Vec<local_environment::Variable>,
    pub server_address: String,
    pub platform: String,
    pub disks: Vec<String>,
    pub program_files: Vec<String>,
    pub http_client: reqwest::Client,
}

impl Contester {
    pub fn new(config: ContesterConfig) -> Result<Self> {
        let invoker_id = hostname::get()
            .map(|h| h.to_string_lossy().into_owned())
            .unwrap_or_else(|_| "undefined".to_string());

        let env = get_local_environment();

        let passwords = get_passwords(&config);
        let mut sandboxes = Vec::with_capacity(passwords.len());
        for (index, password) in passwords.iter().enumerate() {
            let local_base = format!("{}/{}", config.path, index);
            let pair = SandboxPair::new(&local_base);

            // Ensure sandbox directories exist
            std::fs::create_dir_all(&pair.compile.lock().unwrap().path)?;
            std::fs::create_dir_all(&pair.run.lock().unwrap().path)?;

            // Set up restricted user and login session for the run sandbox
            #[cfg(windows)]
            {
                let restricted_user = format!("tester{}", index);
                if let Err(e) = ensure_restricted_user(&restricted_user, password) {
                    tracing::warn!("ensure_restricted_user({:?}): {}", restricted_user, e);
                }
                set_acl(&pair.run.lock().unwrap().path, &restricted_user);

                match WindowsLoginSession::new(&restricted_user, password) {
                    Ok(session) => {
                        pair.run.lock().unwrap().login = Some(session);
                    }
                    Err(e) => {
                        tracing::warn!(
                            "Login failed for {:?}: {}, trying password reset",
                            restricted_user,
                            e
                        );
                        if let Err(e2) = set_local_user_password(&restricted_user, password) {
                            tracing::error!("Password reset failed: {}", e2);
                        } else {
                            match WindowsLoginSession::new(&restricted_user, password) {
                                Ok(session) => {
                                    pair.run.lock().unwrap().login = Some(session);
                                }
                                Err(e2) => {
                                    tracing::error!(
                                        "Login still failed for {:?}: {}",
                                        restricted_user,
                                        e2
                                    );
                                }
                            }
                        }
                    }
                }
            }

            sandboxes.push(pair);
        }

        let (platform, disks, program_files) = platform_info();

        Ok(Self {
            invoker_id,
            sandboxes,
            env,
            server_address: config.server,
            platform,
            disks,
            program_files,
            http_client: reqwest::Client::new(),
        })
    }

    // ── RPC dispatch ────────────────────────────────────────────────────

    /// Dispatch an RPC request to the appropriate handler.
    pub async fn dispatch<R, W>(
        &self,
        codec: &mut ServerCodec<R, W>,
        request: RpcRequest,
    ) -> Result<()>
    where
        R: tokio::io::AsyncRead + Unpin,
        W: tokio::io::AsyncWrite + Unpin,
    {
        let method = &request.method;
        let seq = request.sequence;

        let result: Result<Vec<u8>> = match method.as_str() {
            "Contester.Identify" => self.handle_identify(&request),
            "Contester.LocalExecute" => self.handle_local_execute(&request).await,
            "Contester.LocalExecuteConnected" => {
                self.handle_local_execute_connected(&request).await
            }
            "Contester.Put" => self.handle_put(&request).await,
            "Contester.Get" => self.handle_get(&request).await,
            "Contester.Stat" => self.handle_stat(&request).await,
            "Contester.Clear" => self.handle_clear(&request).await,
            "Contester.GridfsCopy" => self.handle_gridfs_copy(&request).await,
            _ => Err(anyhow::anyhow!("unknown method: {method}")),
        };

        match result {
            Ok(response_bytes) => {
                // Write raw response bytes as a "generic" protobuf message
                let wrapper = RawMessage(response_bytes);
                codec.write_response(method, seq, &wrapper).await?;
            }
            Err(e) => {
                codec.write_error(method, seq, &e.to_string()).await?;
            }
        }
        Ok(())
    }

    // ── Identify ────────────────────────────────────────────────────────

    fn handle_identify(&self, _request: &RpcRequest) -> Result<Vec<u8>> {
        let response = IdentifyResponse {
            invoker_id: self.invoker_id.clone(),
            environment: Some(LocalEnvironment {
                variable: self.env.clone(),
                empty: false,
            }),
            sandboxes: self
                .sandboxes
                .iter()
                .map(|p| SandboxLocations {
                    compile: p.compile.lock().unwrap().path.clone(),
                    run: p.run.lock().unwrap().path.clone(),
                })
                .collect(),
            platform: self.platform.clone(),
            path_separator: std::path::MAIN_SEPARATOR.to_string(),
            disks: self.disks.clone(),
            program_files: self.program_files.clone(),
        };
        Ok(response.encode_to_vec())
    }

    // ── LocalExecute ────────────────────────────────────────────────────

    async fn handle_local_execute(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<LocalExecutionParameters>(request)?;

        let sandbox = find_sandbox(&self.sandboxes, &params)?;
        let guard = sandbox.lock().unwrap();

        let sub = self.setup_subprocess(&params, &guard, true)?;
        let result = tokio::task::spawn_blocking(move || sub.execute()).await??;

        let mut response = LocalExecutionResult::default();
        fill_result(&result, &mut response);
        Ok(response.encode_to_vec())
    }

    // ── LocalExecuteConnected ───────────────────────────────────────────

    async fn handle_local_execute_connected(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<LocalExecuteConnected>(request)?;

        let first_params = params.first.as_ref().ok_or_else(|| anyhow::anyhow!("missing first"))?;
        let second_params = params.second.as_ref().ok_or_else(|| anyhow::anyhow!("missing second"))?;

        let first_sandbox = find_sandbox(&self.sandboxes, first_params)?;
        let second_sandbox = find_sandbox(&self.sandboxes, second_params)?;

        let guard1 = first_sandbox.lock().unwrap();
        let guard2 = second_sandbox.lock().unwrap();

        let mut first = self.setup_subprocess(first_params, &guard1, false)?;
        let mut second = self.setup_subprocess(second_params, &guard2, false)?;

        contester_subprocess::interconnect::interconnect(
            &mut first,
            &mut second,
            None,
            None,
            None,
            None,
        )?;

        let (r1, r2) = tokio::task::spawn_blocking(move || {
            let h2 = std::thread::spawn(move || second.execute());
            let r1 = first.execute();
            let r2 = h2.join().unwrap();
            (r1, r2)
        })
        .await?;

        let mut response = LocalExecuteConnectedResult::default();

        if let Ok(result) = r1 {
            let mut first_result = LocalExecutionResult::default();
            fill_result(&result, &mut first_result);
            response.first = Some(first_result);
        }
        if let Ok(result) = r2 {
            let mut second_result = LocalExecutionResult::default();
            fill_result(&result, &mut second_result);
            response.second = Some(second_result);
        }

        Ok(response.encode_to_vec())
    }

    // ── Put ─────────────────────────────────────────────────────────────

    async fn handle_put(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<FileBlob>(request)?;

        let (resolved, sandbox) = resolve_path(&self.sandboxes, &params.name, true)?;
        let _guard = sandbox.map(|s| s.lock().unwrap());

        // Decode blob data
        let data = match params.data {
            Some(ref blob) => blob.bytes()?,
            None => Vec::new(),
        };

        tokio::fs::write(&resolved, &data).await?;

        let resolved_clone = resolved.clone();
        let stat = tokio::task::spawn_blocking(move || {
            contester_tools::stat_file(Path::new(&resolved_clone), true)
        }).await??;
        let response = tools_stat_to_proto(&stat);
        Ok(response.encode_to_vec())
    }

    // ── Get ─────────────────────────────────────────────────────────────

    async fn handle_get(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<GetRequest>(request)?;

        let (resolved, sandbox) = resolve_path(&self.sandboxes, &params.name, false)?;
        let _guard = sandbox.map(|s| s.lock().unwrap());

        let data = tokio::fs::read(&resolved).await?;
        let blob = Blob::new(&data)?;

        let response = FileBlob {
            name: resolved,
            data: blob,
        };
        Ok(response.encode_to_vec())
    }

    // ── Stat ────────────────────────────────────────────────────────────

    async fn handle_stat(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<StatRequest>(request)?;

        // Resolve and expand all paths (cheap, sync-safe).
        let mut all_paths = Vec::new();
        for name in &params.name {
            let (resolved, _) = resolve_path(&self.sandboxes, name, false)?;
            if params.expand {
                all_paths.extend(glob_paths(&resolved)?);
            } else {
                all_paths.push(resolved);
            }
        }

        // stat_file reads files + computes SHA-1 — run on blocking threadpool.
        let calculate_checksum = params.calculate_checksum;
        let entries = tokio::task::spawn_blocking(move || {
            let mut entries = Vec::new();
            for path in all_paths {
                match contester_tools::stat_file(Path::new(&path), calculate_checksum) {
                    Ok(stat) => entries.push(tools_stat_to_proto(&stat)),
                    Err(_) => {} // file may not exist after glob
                }
            }
            entries
        }).await?;

        let response = FileStats { entries };
        Ok(response.encode_to_vec())
    }

    // ── Clear ───────────────────────────────────────────────────────────

    async fn handle_clear(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<ClearSandboxRequest>(request)?;

        let sandbox = get_sandbox_by_id(&self.sandboxes, &params.sandbox)?;
        let path = sandbox.lock().unwrap().path.clone();

        for retry in 0..10 {
            let p = path.clone();
            match tokio::task::spawn_blocking(move || try_clear_path(&p)).await? {
                Ok(false) => break, // already empty
                Ok(true) => break,  // cleared
                Err(e) => {
                    if retry == 9 {
                        return Err(e);
                    }
                    tracing::error!("clear retry {}: {}", retry, e);
                    tokio::time::sleep(std::time::Duration::from_millis(500)).await;
                }
            }
        }

        let response = EmptyMessage {};
        Ok(response.encode_to_vec())
    }

    // ── GridfsCopy ──────────────────────────────────────────────────────

    async fn handle_gridfs_copy(&self, request: &RpcRequest) -> Result<Vec<u8>> {
        let params = decode_payload::<CopyOperations>(request)?;

        let mut entries = Vec::new();
        for item in &params.entries {
            if item.local_file_name.is_empty() || item.remote_location.is_empty() {
                continue;
            }

            let (resolved, _) = resolve_path(&self.sandboxes, &item.local_file_name, false)?;

            match contester_storage::filer_copy(
                &self.http_client,
                &resolved,
                &item.remote_location,
                item.upload,
                &item.checksum,
                &item.module_type,
                &item.authorization_token,
            )
            .await
            {
                Ok(Some(stat)) => entries.push(tools_stat_to_proto(&stat)),
                Ok(None) => {}
                Err(e) => {
                    tracing::error!("gridfs copy error: {}", e);
                }
            }
        }

        let response = FileStats { entries };
        Ok(response.encode_to_vec())
    }

    // ── Subprocess setup ────────────────────────────────────────────────

    fn setup_subprocess(
        &self,
        params: &LocalExecutionParameters,
        sandbox: &Sandbox,
        do_redirects: bool,
    ) -> Result<Subprocess> {
        let mut sub = Subprocess::default();

        sub.cmd.application_name = if params.application_name.is_empty() { None } else { Some(params.application_name.clone()) };
        sub.cmd.command_line = if params.command_line.is_empty() { None } else { Some(params.command_line.clone()) };
        sub.cmd.parameters = params.command_line_parameters.clone();
        sub.current_directory = if params.current_directory.is_empty() { None } else { Some(params.current_directory.clone()) };

        sub.time_limit = du_from_micros(params.time_limit_micros);
        sub.kernel_time_limit = du_from_micros(params.kernel_time_limit_micros);
        sub.wall_time_limit = du_from_micros(params.wall_time_limit_micros);
        sub.memory_limit = params.memory_limit;
        sub.check_idleness = params.check_idleness;
        sub.restrict_ui = params.restrict_ui;
        sub.no_job = params.no_job;

        if let Some(ref env) = params.environment {
            sub.environment = env
                .variable
                .iter()
                .map(|v| format!("{}={}", v.name, v.value))
                .collect();
            sub.no_inherit_environment = true;
        }

        if do_redirects {
            sub.stdin = fill_redirect(params.std_in.as_ref());
            sub.stdout = fill_redirect(params.std_out.as_ref());
        }

        if params.join_stdout_stderr {
            sub.join_stdout_stderr = true;
        } else {
            sub.stderr = fill_redirect(params.std_err.as_ref());
        }

        // Set login credentials from sandbox (for user impersonation)
        #[cfg(windows)]
        if let Some(ref session) = sandbox.login {
            sub.login = Some(session.to_login_info());
        }

        Ok(sub)
    }
}

// ── Helpers ─────────────────────────────────────────────────────────────────

fn fill_redirect(r: Option<&RedirectParameters>) -> Option<Redirect> {
    let r = r?;
    if !r.filename.is_empty() {
        Some(Redirect {
            filename: Some(r.filename.clone()),
            mode: RedirectMode::File,
            ..Default::default()
        })
    } else if r.memory {
        let data = r
            .buffer
            .as_ref()
            .and_then(|b| b.bytes().ok())
            .unwrap_or_default();
        Some(Redirect {
            mode: RedirectMode::Memory,
            data,
            ..Default::default()
        })
    } else {
        None
    }
}

fn fill_result(result: &SubprocessResult, response: &mut LocalExecutionResult) {
    response.total_processes = result.total_processes;
    response.return_code = result.exit_code;
    response.flags = parse_success_code(result.success_code);
    response.time = parse_time(result);
    response.memory = result.peak_memory;
    response.std_out = Blob::new(&result.output).ok().flatten();
    response.std_err = Blob::new(&result.error).ok().flatten();
}

fn parse_success_code(succ: ExecutionFlags) -> Option<ExecutionResultFlags> {
    if succ.is_empty() {
        return None;
    }
    Some(ExecutionResultFlags {
        killed: succ.contains(ExecutionFlags::KILLED),
        time_limit_hit: succ.contains(ExecutionFlags::TIME_LIMIT_HIT),
        kernel_time_limit_hit: succ.contains(ExecutionFlags::KERNEL_TIME_LIMIT_HIT),
        wall_time_limit_hit: succ.contains(ExecutionFlags::WALL_TIME_LIMIT_HIT),
        memory_limit_hit: succ.contains(ExecutionFlags::MEMORY_LIMIT_HIT),
        inactive: succ.contains(ExecutionFlags::INACTIVE),
        time_limit_hit_post: succ.contains(ExecutionFlags::TIME_LIMIT_HIT_POST),
        kernel_time_limit_hit_post: succ.contains(ExecutionFlags::KERNEL_TIME_LIMIT_HIT_POST),
        memory_limit_hit_post: succ.contains(ExecutionFlags::MEMORY_LIMIT_HIT_POST),
        process_limit_hit: succ.contains(ExecutionFlags::PROCESS_LIMIT_HIT),
        stdout_overflow: false,
        stderr_overflow: false,
        stdpipe_timeout: false,
        stopped_by_signal: false,
        killed_by_signal: false,
    })
}

fn parse_time(r: &SubprocessResult) -> Option<ExecutionResultTime> {
    let ut = get_micros(r.time.user_time);
    let kt = get_micros(r.time.kernel_time);
    let wt = get_micros(r.time.wall_time);
    if ut == 0 && kt == 0 && wt == 0 {
        return None;
    }
    Some(ExecutionResultTime {
        user_time_micros: ut,
        kernel_time_micros: kt,
        wall_time_micros: wt,
    })
}

fn decode_payload<M: Message + Default>(request: &RpcRequest) -> Result<M> {
    match &request.payload {
        Some(data) => Ok(M::decode(data.as_slice())?),
        None => Ok(M::default()),
    }
}

fn try_clear_path(path: &str) -> Result<bool> {
    let entries: Vec<_> = std::fs::read_dir(path)?
        .collect::<std::io::Result<Vec<_>>>()?;

    if entries.is_empty() {
        return Ok(false);
    }

    for entry in entries {
        let fullpath = entry.path();
        if fullpath.is_dir() {
            std::fs::remove_dir_all(&fullpath)?;
        } else {
            std::fs::remove_file(&fullpath)?;
        }
    }
    Ok(true)
}

fn glob_paths(pattern: &str) -> Result<Vec<String>> {
    let mut results = Vec::new();
    for entry in glob::glob(pattern)? {
        results.push(entry?.to_string_lossy().into_owned());
    }
    Ok(results)
}

fn get_local_environment() -> Vec<local_environment::Variable> {
    std::env::vars()
        .map(|(k, v)| local_environment::Variable { name: k, value: v, expand: false })
        .collect()
}

fn get_passwords(config: &ContesterConfig) -> Vec<String> {
    if !config.passwords.is_empty() {
        return config.passwords.split(' ').map(String::from).collect();
    }
    let cores = if config.sandbox_count > 0 {
        config.sandbox_count
    } else {
        let cpus = num_cpus::get();
        std::cmp::max(1, cpus / 2 - 1)
    };

    // Deterministic password generation based on hostname
    use std::hash::{Hash, Hasher};
    use std::collections::hash_map::DefaultHasher;
    let hostname = hostname::get()
        .map(|h| h.to_string_lossy().into_owned())
        .unwrap_or_default();
    let mut hasher = DefaultHasher::new();
    hostname.hash(&mut hasher);
    let mut seed = hasher.finish();

    let alphabet = b"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
    (0..cores)
        .map(|_| {
            let pw: String = (0..8)
                .map(|_| {
                    seed = seed.wrapping_mul(6364136223846793005).wrapping_add(1);
                    alphabet[(seed >> 33) as usize % alphabet.len()] as char
                })
                .collect();
            pw
        })
        .collect()
}

#[cfg(windows)]
fn platform_info() -> (String, Vec<String>, Vec<String>) {
    (
        "win32".to_string(),
        vec!["C:\\".to_string()],
        vec![
            "C:\\Program Files".to_string(),
            "C:\\Program Files (x86)".to_string(),
        ],
    )
}

#[cfg(not(windows))]
fn platform_info() -> (String, Vec<String>, Vec<String>) {
    ("linux".to_string(), vec!["/".to_string()], vec![])
}

/// Wrapper to write pre-encoded bytes as a prost Message.
#[derive(Debug)]
struct RawMessage(Vec<u8>);

impl prost::Message for RawMessage {
    fn encode_raw(&self, buf: &mut impl prost::bytes::BufMut) {
        buf.put_slice(&self.0);
    }

    fn merge_field(
        &mut self,
        _tag: u32,
        _wire_type: prost::encoding::WireType,
        _buf: &mut impl prost::bytes::Buf,
        _ctx: prost::encoding::DecodeContext,
    ) -> Result<(), prost::DecodeError> {
        Ok(())
    }

    fn encoded_len(&self) -> usize {
        self.0.len()
    }

    fn clear(&mut self) {
        self.0.clear();
    }
}

fn find_sandbox<'a>(
    sandboxes: &'a [SandboxPair],
    params: &LocalExecutionParameters,
) -> Result<&'a Arc<std::sync::Mutex<Sandbox>>> {
    if !params.sandbox_id.is_empty() {
        get_sandbox_by_id(sandboxes, &params.sandbox_id)
    } else {
        get_sandbox_by_path(sandboxes, &params.current_directory)
    }
}

// ── Windows user management ─────────────────────────────────────────────────

#[cfg(windows)]
fn to_wide(s: &str) -> Vec<u16> {
    s.encode_utf16().chain(std::iter::once(0)).collect()
}

/// Ensure a restricted local user exists. Creates the user if needed.
#[cfg(windows)]
fn ensure_restricted_user(username: &str, password: &str) -> Result<()> {
    #[repr(C)]
    struct UserInfo1 {
        usri1_name: *mut u16,
        usri1_password: *mut u16,
        usri1_password_age: u32,
        usri1_priv: u32,
        usri1_home_dir: *mut u16,
        usri1_comment: *mut u16,
        usri1_flags: u32,
        usri1_script_path: *mut u16,
    }

    const USER_PRIV_USER: u32 = 1;
    const UF_SCRIPT: u32 = 0x0001;
    const UF_DONT_EXPIRE_PASSWD: u32 = 0x10000;
    const NERR_SUCCESS: u32 = 0;
    const NERR_USER_EXISTS: u32 = 2224;

    #[link(name = "netapi32")]
    unsafe extern "system" {
        fn NetUserAdd(
            servername: *const u16,
            level: u32,
            buf: *const u8,
            parm_err: *mut u32,
        ) -> u32;
    }

    let mut name_wide = to_wide(username);
    let mut password_wide = to_wide(password);

    let info = UserInfo1 {
        usri1_name: name_wide.as_mut_ptr(),
        usri1_password: password_wide.as_mut_ptr(),
        usri1_password_age: 0,
        usri1_priv: USER_PRIV_USER,
        usri1_home_dir: std::ptr::null_mut(),
        usri1_comment: std::ptr::null_mut(),
        usri1_flags: UF_SCRIPT | UF_DONT_EXPIRE_PASSWD,
        usri1_script_path: std::ptr::null_mut(),
    };

    let mut parm_err: u32 = 0;
    let result = unsafe {
        NetUserAdd(
            std::ptr::null(),
            1,
            &info as *const _ as *const u8,
            &mut parm_err,
        )
    };

    match result {
        NERR_SUCCESS | NERR_USER_EXISTS => Ok(()),
        other => Err(anyhow::anyhow!(
            "NetUserAdd({:?}): error code {}",
            username,
            other
        )),
    }
}

/// Set ACL on a sandbox path using subinacl.exe (best-effort).
#[cfg(windows)]
fn set_acl(path: &str, username: &str) {
    let _ = std::process::Command::new("subinacl.exe")
        .args(["/file", path, &format!("/grant={}=RWC", username)])
        .output();
}

/// Reset a local user's password using NetUserSetInfo.
#[cfg(windows)]
fn set_local_user_password(username: &str, password: &str) -> Result<()> {
    #[repr(C)]
    struct UserInfo1003 {
        usri1003_password: *mut u16,
    }

    #[link(name = "netapi32")]
    unsafe extern "system" {
        fn NetUserSetInfo(
            servername: *const u16,
            username: *const u16,
            level: u32,
            buf: *const u8,
            parm_err: *mut u32,
        ) -> u32;
    }

    let name_wide = to_wide(username);
    let mut password_wide = to_wide(password);

    let info = UserInfo1003 {
        usri1003_password: password_wide.as_mut_ptr(),
    };

    let mut parm_err: u32 = 0;
    let result = unsafe {
        NetUserSetInfo(
            std::ptr::null(),
            name_wide.as_ptr(),
            1003,
            &info as *const _ as *const u8,
            &mut parm_err,
        )
    };

    if result != 0 {
        return Err(anyhow::anyhow!(
            "NetUserSetInfo({:?}): error code {}",
            username,
            result
        ));
    }
    Ok(())
}
