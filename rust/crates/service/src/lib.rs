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
    Redirect, RedirectMode, Subprocess, SubprocessResult,
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
            hardware: Some(get_hardware_info()),
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
    response.flags = to_proto_flags(&result.flags);
    response.time = parse_time(result);
    response.memory = result.peak_memory;
    response.std_out = Blob::new(&result.output).ok().flatten();
    response.std_err = Blob::new(&result.error).ok().flatten();
}

fn to_proto_flags(f: &contester_subprocess::ExecutionFlags) -> Option<ExecutionResultFlags> {
    if f.is_clean() {
        return None;
    }
    Some(ExecutionResultFlags {
        killed: f.killed,
        time_limit_hit: f.time_limit_hit,
        kernel_time_limit_hit: f.kernel_time_limit_hit,
        wall_time_limit_hit: f.wall_time_limit_hit,
        memory_limit_hit: f.memory_limit_hit,
        inactive: f.inactive,
        time_limit_hit_post: f.time_limit_hit_post,
        kernel_time_limit_hit_post: f.kernel_time_limit_hit_post,
        memory_limit_hit_post: f.memory_limit_hit_post,
        process_limit_hit: f.process_limit_hit,
        stdout_overflow: f.stdout_overflow,
        stderr_overflow: f.stderr_overflow,
        stdpipe_timeout: f.stdpipe_timeout,
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

// ── Hardware info ────────────────────────────────────────────────────────────

#[cfg(windows)]
fn get_hardware_info() -> HardwareInfo {
    #[repr(C)]
    struct MemoryStatusEx {
        dw_length: u32,
        dw_memory_load: u32,
        ull_total_phys: u64,
        ull_avail_phys: u64,
        ull_total_page_file: u64,
        ull_avail_page_file: u64,
        ull_total_virtual: u64,
        ull_avail_virtual: u64,
        ull_avail_extended_virtual: u64,
    }

    unsafe extern "system" {
        fn GlobalMemoryStatusEx(lp_buffer: *mut MemoryStatusEx) -> i32;
    }

    let mut mem_status: MemoryStatusEx = unsafe { std::mem::zeroed() };
    mem_status.dw_length = std::mem::size_of::<MemoryStatusEx>() as u32;
    unsafe { GlobalMemoryStatusEx(&mut mem_status) };

    let cpu_model = cpuid_brand_string();
    let cpu_mhz = nt_processor_mhz();
    let (cores, threads) = logical_processor_counts();

    HardwareInfo {
        cpu_model,
        cpu_frequency_mhz: cpu_mhz,
        cpu_cores: cores,
        cpu_threads: threads,
        ram_bytes: mem_status.ull_total_phys,
    }
}

/// Read the 48-byte processor brand string via CPUID leaves 0x80000002–0x80000004.
#[cfg(windows)]
fn cpuid_brand_string() -> String {
    #[cfg(target_arch = "x86_64")]
    use std::arch::x86_64::__cpuid;
    #[cfg(target_arch = "x86")]
    use std::arch::x86::__cpuid;

    // Check extended CPUID support
    let ext = unsafe { __cpuid(0x80000000) };
    if ext.eax < 0x80000004 {
        return String::new();
    }

    let mut brand = [0u8; 48];
    for (i, leaf) in (0x80000002..=0x80000004).enumerate() {
        let result = unsafe { __cpuid(leaf) };
        let offset = i * 16;
        brand[offset..offset + 4].copy_from_slice(&result.eax.to_le_bytes());
        brand[offset + 4..offset + 8].copy_from_slice(&result.ebx.to_le_bytes());
        brand[offset + 8..offset + 12].copy_from_slice(&result.ecx.to_le_bytes());
        brand[offset + 12..offset + 16].copy_from_slice(&result.edx.to_le_bytes());
    }

    // Convert to string, trim nulls and whitespace
    String::from_utf8_lossy(&brand)
        .trim_end_matches('\0')
        .trim()
        .to_string()
}

/// Read max MHz via CallNtPowerInformation(ProcessorInformation).
#[cfg(windows)]
fn nt_processor_mhz() -> u32 {
    #[repr(C)]
    struct ProcessorPowerInformation {
        number: u32,
        max_mhz: u32,
        current_mhz: u32,
        mhz_limit: u32,
        max_idle_state: u32,
        current_idle_state: u32,
    }

    const PROCESSOR_INFORMATION: u32 = 11;

    #[link(name = "powrprof")]
    unsafe extern "system" {
        fn CallNtPowerInformation(
            information_level: u32,
            input_buffer: *const std::ffi::c_void,
            input_buffer_length: u32,
            output_buffer: *mut std::ffi::c_void,
            output_buffer_length: u32,
        ) -> i32;
    }

    // Query for one processor — max_mhz is the same across all cores.
    let mut info: ProcessorPowerInformation = unsafe { std::mem::zeroed() };
    let status = unsafe {
        CallNtPowerInformation(
            PROCESSOR_INFORMATION,
            std::ptr::null(),
            0,
            &mut info as *mut _ as *mut std::ffi::c_void,
            std::mem::size_of::<ProcessorPowerInformation>() as u32,
        )
    };
    if status == 0 {
        info.max_mhz
    } else {
        0
    }
}

/// Count physical cores and logical threads via GetLogicalProcessorInformationEx.
#[cfg(windows)]
fn logical_processor_counts() -> (u32, u32) {
    // RelationProcessorCore = 0
    const RELATION_PROCESSOR_CORE: u32 = 0;

    unsafe extern "system" {
        fn GetLogicalProcessorInformationEx(
            relationship: u32,
            buffer: *mut u8,
            returned_length: *mut u32,
        ) -> i32;
    }

    // First call: get required buffer size.
    let mut len: u32 = 0;
    unsafe { GetLogicalProcessorInformationEx(RELATION_PROCESSOR_CORE, std::ptr::null_mut(), &mut len) };
    if len == 0 {
        return (0, 0);
    }

    let mut buf = vec![0u8; len as usize];
    let ok = unsafe {
        GetLogicalProcessorInformationEx(RELATION_PROCESSOR_CORE, buf.as_mut_ptr(), &mut len)
    };
    if ok == 0 {
        return (0, 0);
    }

    // Walk the variable-length entries. Each entry for RelationProcessorCore
    // represents one physical core. The GROUP_AFFINITY mask bits give the
    // logical threads on that core.
    let mut cores: u32 = 0;
    let mut threads: u32 = 0;
    let mut offset: usize = 0;
    while offset + 4 < len as usize {
        // Entry layout: u32 Relationship (offset 0), u32 Size (offset 4), ...
        let size = u32::from_le_bytes([
            buf[offset + 4],
            buf[offset + 5],
            buf[offset + 6],
            buf[offset + 7],
        ]) as usize;
        if size == 0 {
            break;
        }

        cores += 1;

        // PROCESSOR_RELATIONSHIP starts at offset+8:
        //   u8 Flags, u8 EfficiencyClass, u8[20] Reserved, u16 GroupCount,
        //   then GroupCount × GROUP_AFFINITY (each 12 bytes: u64 Mask + u16 Group + u16[3] Reserved)
        // GROUP_AFFINITY.Mask is at offset+8+24 = offset+32
        if offset + 40 <= len as usize {
            let mask = u64::from_le_bytes([
                buf[offset + 32],
                buf[offset + 33],
                buf[offset + 34],
                buf[offset + 35],
                buf[offset + 36],
                buf[offset + 37],
                buf[offset + 38],
                buf[offset + 39],
            ]);
            threads += mask.count_ones();
        }

        offset += size;
    }

    (cores, threads)
}

#[cfg(not(windows))]
fn get_hardware_info() -> HardwareInfo {
    use std::fs;

    // Parse /proc/cpuinfo for model and MHz
    let mut cpu_model = String::new();
    let mut cpu_mhz: u32 = 0;
    let mut cpu_threads: u32 = 0;

    if let Ok(cpuinfo) = fs::read_to_string("/proc/cpuinfo") {
        for line in cpuinfo.lines() {
            if cpu_model.is_empty() && line.starts_with("model name") {
                if let Some(val) = line.split(':').nth(1) {
                    cpu_model = val.trim().to_string();
                }
            }
            if cpu_mhz == 0 && line.starts_with("cpu MHz") {
                if let Some(val) = line.split(':').nth(1) {
                    cpu_mhz = val.trim().parse::<f64>().unwrap_or(0.0) as u32;
                }
            }
            if line.starts_with("processor") {
                cpu_threads += 1;
            }
        }
    }

    // Total RAM from /proc/meminfo
    let mut ram_bytes: u64 = 0;
    if let Ok(meminfo) = fs::read_to_string("/proc/meminfo") {
        for line in meminfo.lines() {
            if line.starts_with("MemTotal:") {
                if let Some(val) = line.split_whitespace().nth(1) {
                    ram_bytes = val.parse::<u64>().unwrap_or(0) * 1024; // kB to bytes
                }
                break;
            }
        }
    }

    HardwareInfo {
        cpu_model,
        cpu_frequency_mhz: cpu_mhz,
        cpu_cores: cpu_threads,
        cpu_threads,
        ram_bytes,
    }
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
