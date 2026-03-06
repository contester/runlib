//! Windows subprocess implementation using Win32 API.
//!
//! Covers process creation (suspended), Job Object configuration,
//! process monitoring (BottomHalf), resource measurement, and user impersonation.

use std::fs::File;
use std::mem::{self, zeroed};
use std::os::windows::io::AsRawHandle;
use std::ptr;
use std::time::Duration;

use anyhow::Result;
use windows_sys::Win32::Foundation::*;
use windows_sys::Win32::System::JobObjects::*;
use windows_sys::Win32::System::ProcessStatus::*;
use windows_sys::Win32::System::Console::{GetStdHandle, STD_ERROR_HANDLE, STD_INPUT_HANDLE, STD_OUTPUT_HANDLE};
use windows_sys::Win32::System::SystemInformation::GetSystemTimeAsFileTime;
use windows_sys::Win32::System::Threading::*;

use crate::redirects::SubprocessData;
use crate::{LoginInfo, RunningState, Subprocess, SubprocessResult, EF_STDERR_OVERFLOW, EF_STDOUT_OVERFLOW};

// ── Constants ────────────────────────────────────────────────────────────────

const STARTF_FORCEOFFFEEDBACK: u32 = 0x00000080;
const SW_SHOWMINNOACTIVE: u16 = 7;

// ── User impersonation constants ─────────────────────────────────────────────

const LOGON32_LOGON_INTERACTIVE: u32 = 2;
const LOGON32_PROVIDER_DEFAULT: u32 = 0;
const LOGON_WITH_PROFILE: u32 = 1;
const PI_NOUI: u32 = 1;

// ── FFI declarations for user impersonation ──────────────────────────────────

#[repr(C)]
struct PROFILEINFOW {
    dw_size: u32,
    dw_flags: u32,
    lp_user_name: *mut u16,
    lp_profile_path: *mut u16,
    lp_default_path: *mut u16,
    lp_server_name: *mut u16,
    lp_policy_path: *mut u16,
    h_profile: HANDLE,
}

#[link(name = "advapi32")]
unsafe extern "system" {
    fn LogonUserW(
        lpszUsername: *const u16,
        lpszDomain: *const u16,
        lpszPassword: *const u16,
        dwLogonType: u32,
        dwLogonProvider: u32,
        phToken: *mut HANDLE,
    ) -> i32;

    fn CreateProcessWithLogonW(
        lpUsername: *const u16,
        lpDomain: *const u16,
        lpPassword: *const u16,
        dwLogonFlags: u32,
        lpApplicationName: *const u16,
        lpCommandLine: *mut u16,
        dwCreationFlags: u32,
        lpEnvironment: *const core::ffi::c_void,
        lpCurrentDirectory: *const u16,
        lpStartupInfo: *const STARTUPINFOW,
        lpProcessInformation: *mut PROCESS_INFORMATION,
    ) -> i32;
}

#[link(name = "userenv")]
unsafe extern "system" {
    fn LoadUserProfileW(
        hToken: HANDLE,
        lpProfileInfo: *mut PROFILEINFOW,
    ) -> i32;

    fn UnloadUserProfile(
        hToken: HANDLE,
        hProfile: HANDLE,
    ) -> i32;
}

// ── WindowsLoginSession ─────────────────────────────────────────────────────

/// A Windows login session obtained via LogonUserW + LoadUserProfileW.
/// Automatically cleans up (UnloadUserProfile + CloseHandle) on drop.
pub struct WindowsLoginSession {
    pub username: String,
    pub password: String,
    h_user: HANDLE,
    h_profile: HANDLE,
}

// SAFETY: The HANDLE values are only used for cleanup in Drop and are not
// shared across threads concurrently. The session is typically stored in
// an Arc<Mutex<Sandbox>> which provides synchronization.
unsafe impl Send for WindowsLoginSession {}
unsafe impl Sync for WindowsLoginSession {}

impl std::fmt::Debug for WindowsLoginSession {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_struct("WindowsLoginSession")
            .field("username", &self.username)
            .field("h_user", &self.h_user)
            .field("h_profile", &self.h_profile)
            .finish()
    }
}

impl WindowsLoginSession {
    /// Create a new login session by calling LogonUserW + LoadUserProfileW.
    pub fn new(username: &str, password: &str) -> Result<Self> {
        let mut session = Self {
            username: username.to_string(),
            password: password.to_string(),
            h_user: INVALID_HANDLE_VALUE,
            h_profile: INVALID_HANDLE_VALUE,
        };

        if username.is_empty() {
            return Ok(session);
        }

        session.prepare()?;
        Ok(session)
    }

    /// Perform LogonUserW + LoadUserProfileW.
    fn prepare(&mut self) -> Result<()> {
        let username_wide = to_wide(&self.username);
        let domain_wide = to_wide(".");
        let password_wide = to_wide(&self.password);

        let result = unsafe {
            LogonUserW(
                username_wide.as_ptr(),
                domain_wide.as_ptr(),
                password_wide.as_ptr(),
                LOGON32_LOGON_INTERACTIVE,
                LOGON32_PROVIDER_DEFAULT,
                &mut self.h_user,
            )
        };

        if result == 0 {
            let err = std::io::Error::last_os_error();
            return Err(anyhow::anyhow!(
                "LogonUserW({:?}): {}",
                self.username,
                err
            ));
        }

        match self.load_profile() {
            Ok(()) => Ok(()),
            Err(e) => {
                unsafe { CloseHandle(self.h_user) };
                self.h_user = INVALID_HANDLE_VALUE;
                Err(e)
            }
        }
    }

    /// Load the user profile via LoadUserProfileW.
    fn load_profile(&mut self) -> Result<()> {
        let mut username_wide = to_wide(&self.username);
        let mut pinfo: PROFILEINFOW = unsafe { zeroed() };
        pinfo.dw_size = mem::size_of::<PROFILEINFOW>() as u32;
        pinfo.dw_flags = PI_NOUI;
        pinfo.lp_user_name = username_wide.as_mut_ptr();

        let result = unsafe { LoadUserProfileW(self.h_user, &mut pinfo) };

        if result == 0 {
            let err = std::io::Error::last_os_error();
            return Err(anyhow::anyhow!(
                "LoadUserProfileW({:?}): {}",
                self.username,
                err
            ));
        }

        self.h_profile = pinfo.h_profile;
        Ok(())
    }

    /// Get a LoginInfo (username + password) for use with Subprocess.
    pub fn to_login_info(&self) -> LoginInfo {
        LoginInfo {
            username: self.username.clone(),
            password: self.password.clone(),
        }
    }
}

impl Drop for WindowsLoginSession {
    fn drop(&mut self) {
        // Unload user profile (retry on failure, matching Go behavior)
        if !self.h_profile.is_null() && self.h_profile != INVALID_HANDLE_VALUE {
            loop {
                let result = unsafe { UnloadUserProfile(self.h_user, self.h_profile) };
                if result != 0 {
                    break;
                }
                let err = std::io::Error::last_os_error();
                tracing::error!("UnloadUserProfile({:?}): {}", self.username, err);
                // Break after logging to avoid infinite loop in practice
                break;
            }
            self.h_profile = INVALID_HANDLE_VALUE;
        }

        // Close logon token
        if !self.h_user.is_null() && self.h_user != INVALID_HANDLE_VALUE {
            unsafe { CloseHandle(self.h_user) };
            self.h_user = INVALID_HANDLE_VALUE;
        }
    }
}

// ── Platform data ────────────────────────────────────────────────────────────

/// Windows-specific runtime data for a subprocess.
pub struct PlatformData {
    pub process: HANDLE,
    pub thread: HANDLE,
    pub job: HANDLE,
}

impl PlatformData {
    pub fn new() -> Self {
        Self {
            process: INVALID_HANDLE_VALUE,
            thread: INVALID_HANDLE_VALUE,
            job: INVALID_HANDLE_VALUE,
        }
    }
}

// ── String helpers ───────────────────────────────────────────────────────────

/// Convert a Rust string to a null-terminated UTF-16 Vec.
fn to_wide(s: &str) -> Vec<u16> {
    s.encode_utf16().chain(std::iter::once(0)).collect()
}

/// Build a UTF-16 environment block from a list of "KEY=VALUE" strings.
/// The block is double-null terminated.
fn build_environment_block(env: &[String]) -> Vec<u16> {
    let mut block = Vec::with_capacity(16 * 1024);
    for var in env {
        block.extend(var.encode_utf16());
        block.push(0);
    }
    block.push(0); // double null terminator
    block
}

// ── Handle helpers ───────────────────────────────────────────────────────────

/// Get the raw HANDLE from an Option<File>, or INVALID_HANDLE_VALUE if None.
fn file_to_handle(f: &Option<File>) -> HANDLE {
    match f {
        Some(file) => file.as_raw_handle() as HANDLE,
        None => INVALID_HANDLE_VALUE,
    }
}

/// Mark a HANDLE as inheritable.
fn set_handle_inheritable(h: HANDLE, inherit: bool) {
    if h != INVALID_HANDLE_VALUE && !h.is_null() {
        let flags = if inherit { HANDLE_FLAG_INHERIT } else { 0 };
        unsafe { SetHandleInformation(h, HANDLE_FLAG_INHERIT, flags) };
    }
}

// ── Time helpers ─────────────────────────────────────────────────────────────

/// Convert a Win32 FILETIME to a Duration.
fn filetime_to_duration(ft: &FILETIME) -> Duration {
    let ns100 = ((ft.dwHighDateTime as u64) << 32) | (ft.dwLowDateTime as u64);
    Duration::from_nanos(ns100 * 100)
}

// ── Process creation ─────────────────────────────────────────────────────────

/// Set up all redirects and populate STARTUPINFOW handles.
fn setup_all_redirects(
    sub: &Subprocess,
    d: &mut SubprocessData,
    si: &mut STARTUPINFOW,
) -> Result<()> {
    let stdin_file = d.setup_input(sub.stdin.as_ref())?;
    let stdout_file = d.setup_output(sub.stdout.as_ref(), false)?;

    let stderr_file = if sub.join_stdout_stderr {
        None
    } else {
        d.setup_output(sub.stderr.as_ref(), true)?
    };

    let h_stdin = file_to_handle(&stdin_file);
    let h_stdout = file_to_handle(&stdout_file);
    let h_stderr = if sub.join_stdout_stderr {
        h_stdout
    } else {
        file_to_handle(&stderr_file)
    };

    // If any redirect is active, set STARTF_USESTDHANDLES
    if h_stdin != INVALID_HANDLE_VALUE
        || h_stdout != INVALID_HANDLE_VALUE
        || h_stderr != INVALID_HANDLE_VALUE
    {
        si.dwFlags |= STARTF_USESTDHANDLES;

        si.hStdInput = if h_stdin != INVALID_HANDLE_VALUE {
            h_stdin
        } else {
            unsafe { GetStdHandle(STD_INPUT_HANDLE) }
        };
        si.hStdOutput = if h_stdout != INVALID_HANDLE_VALUE {
            h_stdout
        } else {
            unsafe { GetStdHandle(STD_OUTPUT_HANDLE) }
        };
        si.hStdError = if h_stderr != INVALID_HANDLE_VALUE {
            h_stderr
        } else {
            unsafe { GetStdHandle(STD_ERROR_HANDLE) }
        };
    }

    // The File objects are already tracked in d.close_after_start via setup_*.
    // We need to forget *these* copies so they don't close the handles prematurely.
    std::mem::forget(stdin_file);
    std::mem::forget(stdout_file);
    std::mem::forget(stderr_file);

    Ok(())
}

/// Create a process in suspended state.
pub fn create_frozen(sub: &Subprocess) -> Result<SubprocessData> {
    let mut d = SubprocessData::new();

    let mut si: STARTUPINFOW = unsafe { zeroed() };
    si.cb = mem::size_of::<STARTUPINFOW>() as u32;
    si.dwFlags = STARTF_FORCEOFFFEEDBACK | STARTF_USESHOWWINDOW;
    si.wShowWindow = SW_SHOWMINNOACTIVE;

    // Set up redirects
    setup_all_redirects(sub, &mut d, &mut si)?;

    // Build command line
    let command_line = if !sub.cmd.command_line.is_empty() {
        sub.cmd.command_line.clone()
    } else if !sub.cmd.parameters.is_empty() {
        let mut parts = Vec::new();
        if !sub.cmd.application_name.is_empty() {
            parts.push(sub.cmd.application_name.clone());
        }
        parts.extend(sub.cmd.parameters.iter().cloned());
        parts.join(" ")
    } else {
        sub.cmd.application_name.clone()
    };

    let app_name_wide = if !sub.cmd.application_name.is_empty() {
        Some(to_wide(&sub.cmd.application_name))
    } else {
        None
    };
    let mut cmd_line_wide = to_wide(&command_line);

    let current_dir_wide = if !sub.current_directory.is_empty() {
        Some(to_wide(&sub.current_directory))
    } else {
        None
    };

    let env_block = if sub.no_inherit_environment {
        Some(build_environment_block(&sub.environment))
    } else {
        None
    };

    let mut pi: PROCESS_INFORMATION = unsafe { zeroed() };

    // Mark handles as inheritable before CreateProcess
    if si.dwFlags & STARTF_USESTDHANDLES != 0 {
        set_handle_inheritable(si.hStdInput, true);
        set_handle_inheritable(si.hStdOutput, true);
        set_handle_inheritable(si.hStdError, true);
    }

    let result = if let Some(ref login) = sub.login {
        // CreateProcessWithLogonW path — run as another user.
        // Uses reduced creation flags (no CREATE_NEW_PROCESS_GROUP,
        // CREATE_NEW_CONSOLE, CREATE_BREAKAWAY_FROM_JOB).
        let username_wide = to_wide(&login.username);
        let domain_wide = to_wide(".");
        let password_wide = to_wide(&login.password);

        let creation_flags = CREATE_SUSPENDED | CREATE_UNICODE_ENVIRONMENT;

        unsafe {
            CreateProcessWithLogonW(
                username_wide.as_ptr(),
                domain_wide.as_ptr(),
                password_wide.as_ptr(),
                LOGON_WITH_PROFILE,
                app_name_wide.as_ref().map_or(ptr::null(), |v| v.as_ptr()),
                cmd_line_wide.as_mut_ptr(),
                creation_flags,
                env_block
                    .as_ref()
                    .map_or(ptr::null(), |v| v.as_ptr() as *const core::ffi::c_void),
                current_dir_wide
                    .as_ref()
                    .map_or(ptr::null(), |v| v.as_ptr()),
                &si,
                &mut pi,
            )
        }
    } else {
        // Standard CreateProcessW path.
        let creation_flags = CREATE_NEW_PROCESS_GROUP
            | CREATE_NEW_CONSOLE
            | CREATE_SUSPENDED
            | CREATE_UNICODE_ENVIRONMENT
            | CREATE_BREAKAWAY_FROM_JOB;

        unsafe {
            CreateProcessW(
                app_name_wide.as_ref().map_or(ptr::null(), |v| v.as_ptr()),
                cmd_line_wide.as_mut_ptr(),
                ptr::null(),
                ptr::null(),
                TRUE, // inherit handles
                creation_flags,
                env_block
                    .as_ref()
                    .map_or(ptr::null(), |v| v.as_ptr() as *const _),
                current_dir_wide
                    .as_ref()
                    .map_or(ptr::null(), |v| v.as_ptr()),
                &si,
                &mut pi,
            )
        }
    };

    // Close parent-side redirect handles
    for f in d.close_after_start.drain(..) {
        drop(f);
    }

    if result == 0 {
        let err = std::io::Error::last_os_error();
        for cleanup in d.cleanup_if_failed.drain(..) {
            cleanup();
        }
        let api_name = if sub.login.is_some() {
            "CreateProcessWithLogonW"
        } else {
            "CreateProcessW"
        };
        return Err(anyhow::anyhow!(
            "{}({:?}): {}",
            api_name,
            command_line,
            err
        ));
    }

    d.platform.process = pi.hProcess;
    d.platform.thread = pi.hThread;
    d.platform.job = INVALID_HANDLE_VALUE;

    // Set process affinity if requested
    if sub.process_affinity_mask != 0 {
        let ok =
            unsafe { SetProcessAffinityMask(d.platform.process, sub.process_affinity_mask as usize) };
        if ok == 0 {
            let err = std::io::Error::last_os_error();
            terminate_and_close(&mut d.platform);
            return Err(anyhow::anyhow!(
                "SetProcessAffinityMask(0b{:b}): {}",
                sub.process_affinity_mask,
                err
            ));
        }
    }

    // Create and configure Job Object
    if !sub.no_job {
        match create_job(sub) {
            Ok(job_handle) => {
                let assign_ok =
                    unsafe { AssignProcessToJobObject(job_handle, d.platform.process) };
                if assign_ok == 0 {
                    let err = std::io::Error::last_os_error();
                    tracing::error!(
                        "AssignProcessToJobObject failed: {}, hJob={:?}, hProcess={:?}",
                        err,
                        job_handle,
                        d.platform.process
                    );
                    unsafe {
                        CloseHandle(job_handle);
                    }
                    if sub.fail_on_job_creation_failure {
                        terminate_and_close(&mut d.platform);
                        return Err(anyhow::anyhow!("AssignProcessToJobObject: {}", err));
                    }
                } else {
                    d.platform.job = job_handle;
                }
            }
            Err(e) => {
                if sub.fail_on_job_creation_failure {
                    terminate_and_close(&mut d.platform);
                    return Err(e.context("CreateJob"));
                }
                tracing::error!("CreateJob: {}", e);
            }
        }
    }

    Ok(d)
}

/// Create and configure a Job Object with limits.
fn create_job(sub: &Subprocess) -> Result<HANDLE> {
    let job = unsafe { CreateJobObjectW(ptr::null(), ptr::null()) };
    if job.is_null() {
        return Err(anyhow::anyhow!(
            "CreateJobObjectW: {}",
            std::io::Error::last_os_error()
        ));
    }

    // UI restrictions
    if sub.restrict_ui {
        let mut info: JOBOBJECT_BASIC_UI_RESTRICTIONS = unsafe { zeroed() };
        info.UIRestrictionsClass = JOB_OBJECT_UILIMIT_DESKTOP
            | JOB_OBJECT_UILIMIT_DISPLAYSETTINGS
            | JOB_OBJECT_UILIMIT_EXITWINDOWS
            | JOB_OBJECT_UILIMIT_GLOBALATOMS
            | JOB_OBJECT_UILIMIT_HANDLES
            | JOB_OBJECT_UILIMIT_READCLIPBOARD
            | JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS
            | JOB_OBJECT_UILIMIT_WRITECLIPBOARD;

        let ok = unsafe {
            SetInformationJobObject(
                job,
                JobObjectBasicUIRestrictions,
                &info as *const _ as *const _,
                mem::size_of::<JOBOBJECT_BASIC_UI_RESTRICTIONS>() as u32,
            )
        };
        if ok == 0 {
            let err = std::io::Error::last_os_error();
            unsafe { CloseHandle(job) };
            return Err(anyhow::anyhow!(
                "SetInformationJobObject(UIRestrictions): {}",
                err
            ));
        }
    }

    // Extended limit information
    let mut einfo: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = unsafe { zeroed() };
    einfo.BasicLimitInformation.LimitFlags =
        JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION | JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;

    // Hard time limit: soft + 1s, or wall time limit
    let hard_time_limit = if sub.time_limit > Duration::ZERO {
        sub.time_limit + Duration::from_secs(1)
    } else {
        sub.wall_time_limit
    };

    if hard_time_limit > Duration::ZERO {
        let ns100 = hard_time_limit.as_nanos() as i64 / 100;
        tracing::debug!("Setting hard time limit: {:?}", hard_time_limit);
        einfo.BasicLimitInformation.PerJobUserTimeLimit = ns100;
        einfo.BasicLimitInformation.PerProcessUserTimeLimit = ns100;
        einfo.BasicLimitInformation.LimitFlags |=
            JOB_OBJECT_LIMIT_PROCESS_TIME | JOB_OBJECT_LIMIT_JOB_TIME;
    }

    if sub.process_limit > 0 {
        einfo.BasicLimitInformation.ActiveProcessLimit = sub.process_limit;
        einfo.BasicLimitInformation.LimitFlags |= JOB_OBJECT_LIMIT_ACTIVE_PROCESS;
    }

    if sub.hard_memory_limit > 0 {
        einfo.ProcessMemoryLimit = sub.hard_memory_limit as usize;
        einfo.JobMemoryLimit = sub.hard_memory_limit as usize;
        einfo.BasicLimitInformation.MaximumWorkingSetSize = sub.hard_memory_limit as usize;
        einfo.BasicLimitInformation.LimitFlags |= JOB_OBJECT_LIMIT_JOB_MEMORY
            | JOB_OBJECT_LIMIT_PROCESS_MEMORY
            | JOB_OBJECT_LIMIT_WORKINGSET;
    }

    let ok = unsafe {
        SetInformationJobObject(
            job,
            JobObjectExtendedLimitInformation,
            &einfo as *const _ as *const _,
            mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
        )
    };
    if ok == 0 {
        let err = std::io::Error::last_os_error();
        unsafe { CloseHandle(job) };
        return Err(anyhow::anyhow!(
            "SetInformationJobObject(ExtendedLimits): {}",
            err
        ));
    }

    Ok(job)
}

// ── Unfreeze ─────────────────────────────────────────────────────────────────

/// Resume the suspended process thread.
pub fn unfreeze(d: &mut SubprocessData) {
    let thread = d.platform.thread;
    if thread == INVALID_HANDLE_VALUE {
        return;
    }

    for retry in 0..10 {
        let count = unsafe { ResumeThread(thread) };
        if count == u32::MAX {
            let err = std::io::Error::last_os_error();
            tracing::error!("ResumeThread failed: {}", err);
            if retry >= 9 {
                panic!("UNSUSPEND FAILED after 10 retries");
            }
            std::thread::sleep(Duration::from_millis(100));
        } else if count <= 1 {
            break;
        } else {
            tracing::error!("ResumeThread: oldcount={}", count);
            if retry >= 9 {
                panic!("UNSUSPEND FAILED after 10 retries");
            }
            std::thread::sleep(Duration::from_millis(100));
        }
    }

    unsafe {
        CloseHandle(thread);
    }
    d.platform.thread = INVALID_HANDLE_VALUE;
}

// ── Resource measurement ─────────────────────────────────────────────────────

/// Update process time statistics.
fn update_process_times(pdata: &PlatformData, result: &mut SubprocessResult, finished: bool) {
    let mut creation: FILETIME = unsafe { zeroed() };
    let mut exit: FILETIME = unsafe { zeroed() };
    let mut kernel: FILETIME = unsafe { zeroed() };
    let mut user: FILETIME = unsafe { zeroed() };

    let ok =
        unsafe { GetProcessTimes(pdata.process, &mut creation, &mut exit, &mut kernel, &mut user) };
    if ok == 0 {
        tracing::error!(
            "GetProcessTimes failed: {}",
            std::io::Error::last_os_error()
        );
        return;
    }

    if !finished {
        unsafe {
            GetSystemTimeAsFileTime(&mut exit);
        }
    }

    result.time.wall_time = filetime_to_duration(&exit)
        .checked_sub(filetime_to_duration(&creation))
        .unwrap_or(Duration::ZERO);

    // Try Job Object accounting (more accurate for multi-process)
    if pdata.job != INVALID_HANDLE_VALUE {
        let mut jinfo: JOBOBJECT_BASIC_ACCOUNTING_INFORMATION = unsafe { zeroed() };
        let mut ret_len: u32 = 0;
        let ok = unsafe {
            QueryInformationJobObject(
                pdata.job,
                JobObjectBasicAccountingInformation,
                &mut jinfo as *mut _ as *mut _,
                mem::size_of::<JOBOBJECT_BASIC_ACCOUNTING_INFORMATION>() as u32,
                &mut ret_len,
            )
        };
        if ok != 0 {
            result.time.user_time = Duration::from_nanos((jinfo.TotalUserTime as u64) * 100);
            result.time.kernel_time = Duration::from_nanos((jinfo.TotalKernelTime as u64) * 100);
            result.total_processes = jinfo.TotalProcesses as u64;
            return;
        }
        tracing::error!(
            "QueryInformationJobObject(Accounting) failed: {}",
            std::io::Error::last_os_error()
        );
    }

    result.time.user_time = filetime_to_duration(&user);
    result.time.kernel_time = filetime_to_duration(&kernel);
}

/// Update peak memory usage.
fn update_process_memory(pdata: &PlatformData, result: &mut SubprocessResult) {
    if pdata.job != INVALID_HANDLE_VALUE {
        let mut jinfo: JOBOBJECT_EXTENDED_LIMIT_INFORMATION = unsafe { zeroed() };
        let mut ret_len: u32 = 0;
        let ok = unsafe {
            QueryInformationJobObject(
                pdata.job,
                JobObjectExtendedLimitInformation,
                &mut jinfo as *mut _ as *mut _,
                mem::size_of::<JOBOBJECT_EXTENDED_LIMIT_INFORMATION>() as u32,
                &mut ret_len,
            )
        };
        if ok != 0 {
            result.peak_memory = jinfo.PeakJobMemoryUsed as u64;
            return;
        }
        tracing::error!(
            "QueryInformationJobObject(ExtendedLimits) failed: {}",
            std::io::Error::last_os_error()
        );
    }

    result.peak_memory = get_process_memory_usage(pdata.process);
}

/// Get memory usage for a single process.
fn get_process_memory_usage(process: HANDLE) -> u64 {
    let mut pmc: PROCESS_MEMORY_COUNTERS_EX = unsafe { zeroed() };
    pmc.cb = mem::size_of::<PROCESS_MEMORY_COUNTERS_EX>() as u32;

    let ok = unsafe {
        GetProcessMemoryInfo(
            process,
            &mut pmc as *mut _ as *mut PROCESS_MEMORY_COUNTERS,
            mem::size_of::<PROCESS_MEMORY_COUNTERS_EX>() as u32,
        )
    };
    if ok == 0 {
        return 0;
    }

    std::cmp::max(pmc.PeakPagefileUsage as u64, pmc.PrivateUsage as u64)
}

// ── Process termination ──────────────────────────────────────────────────────

/// Repeatedly terminate a process until it's confirmed dead.
fn loop_terminate(process: HANDLE) {
    loop {
        unsafe {
            TerminateProcess(process, 0);
        }
        let wait = unsafe { WaitForSingleObject(process, 1000) };
        if wait != WAIT_TIMEOUT {
            break;
        }
    }
}

/// Terminate process and close process/thread handles.
fn terminate_and_close(pdata: &mut PlatformData) {
    if pdata.process != INVALID_HANDLE_VALUE {
        loop_terminate(pdata.process);
        unsafe {
            CloseHandle(pdata.process);
        }
        pdata.process = INVALID_HANDLE_VALUE;
    }
    if pdata.thread != INVALID_HANDLE_VALUE {
        unsafe {
            CloseHandle(pdata.thread);
        }
        pdata.thread = INVALID_HANDLE_VALUE;
    }
}

/// Terminate a frozen (suspended) process and clean up all handles.
/// Called when DLL injection or other post-creation setup fails.
pub fn terminate_frozen(d: &mut SubprocessData) {
    terminate_and_close(&mut d.platform);
    if d.platform.job != INVALID_HANDLE_VALUE {
        unsafe { CloseHandle(d.platform.job) };
        d.platform.job = INVALID_HANDLE_VALUE;
    }
}

// ── DLL Injection ────────────────────────────────────────────────────────────

// FFI for DLL injection (all from kernel32.dll, already linked by windows-sys)
#[link(name = "kernel32")]
unsafe extern "system" {
    fn VirtualAllocEx(
        hProcess: HANDLE,
        lpAddress: usize,
        dwSize: usize,
        flAllocationType: u32,
        flProtect: u32,
    ) -> usize;

    fn VirtualFreeEx(
        hProcess: HANDLE,
        lpAddress: usize,
        dwSize: usize,
        dwFreeType: u32,
    ) -> i32;

    fn WriteProcessMemory(
        hProcess: HANDLE,
        lpBaseAddress: usize,
        lpBuffer: *const u8,
        nSize: usize,
        lpNumberOfBytesWritten: *mut usize,
    ) -> i32;

    fn CreateRemoteThread(
        hProcess: HANDLE,
        lpThreadAttributes: *const core::ffi::c_void,
        dwStackSize: usize,
        lpStartAddress: usize,
        lpParameter: usize,
        dwCreationFlags: u32,
        lpThreadId: *mut u32,
    ) -> HANDLE;

    fn GetModuleHandleW(lpModuleName: *const u16) -> HANDLE;

    fn GetProcAddress(hModule: HANDLE, lpProcName: *const u8) -> usize;
}

const MEM_COMMIT: u32 = 0x1000;
const MEM_RELEASE: u32 = 0x8000;
const PAGE_READWRITE: u32 = 0x04;

/// Inject a DLL into a suspended process by creating a remote thread
/// that calls LoadLibraryW with the DLL path.
pub fn inject_dll(d: &SubprocessData, dll: &str) -> Result<()> {
    let process = d.platform.process;

    // 1. Resolve LoadLibraryW address in our process.
    //    Since kernel32.dll is loaded at the same base address in all processes
    //    (ASLR is per-boot, not per-process), this address is valid in the target too.
    let kernel32_name = to_wide("kernel32.dll");
    let kernel32 = unsafe { GetModuleHandleW(kernel32_name.as_ptr()) };
    if kernel32.is_null() {
        return Err(anyhow::anyhow!(
            "GetModuleHandleW(kernel32): {}",
            std::io::Error::last_os_error()
        ));
    }

    let load_library_name = b"LoadLibraryW\0";
    let load_library_addr =
        unsafe { GetProcAddress(kernel32, load_library_name.as_ptr()) };
    if load_library_addr == 0 {
        return Err(anyhow::anyhow!(
            "GetProcAddress(LoadLibraryW): {}",
            std::io::Error::last_os_error()
        ));
    }

    tracing::debug!(
        "InjectDll: injecting {:?} via LoadLibraryW at {:#x}",
        dll,
        load_library_addr
    );

    // 2. Write the DLL path (UTF-16) into the target process
    let dll_wide = to_wide(dll);
    let dll_bytes_len = dll_wide.len() * 2; // size in bytes

    let remote_buf = unsafe {
        VirtualAllocEx(process, 0, dll_bytes_len, MEM_COMMIT, PAGE_READWRITE)
    };
    if remote_buf == 0 {
        return Err(anyhow::anyhow!(
            "VirtualAllocEx({}): {}",
            dll_bytes_len,
            std::io::Error::last_os_error()
        ));
    }

    let result = inject_dll_inner(process, remote_buf, &dll_wide, load_library_addr);

    // Always free remote buffer, regardless of success/failure
    unsafe { VirtualFreeEx(process, remote_buf, 0, MEM_RELEASE) };

    result
}

/// Inner function for inject_dll — write memory, create remote thread, wait.
/// Separated so the caller can handle VirtualFreeEx cleanup unconditionally.
fn inject_dll_inner(
    process: HANDLE,
    remote_buf: usize,
    dll_wide: &[u16],
    load_library_addr: usize,
) -> Result<()> {
    let dll_bytes_len = dll_wide.len() * 2;

    let mut bytes_written: usize = 0;
    let ok = unsafe {
        WriteProcessMemory(
            process,
            remote_buf,
            dll_wide.as_ptr() as *const u8,
            dll_bytes_len,
            &mut bytes_written,
        )
    };
    if ok == 0 {
        return Err(anyhow::anyhow!(
            "WriteProcessMemory: {}",
            std::io::Error::last_os_error()
        ));
    }

    // Create a remote thread that calls LoadLibraryW(remote_buf)
    let mut thread_id: u32 = 0;
    let thread = unsafe {
        CreateRemoteThread(
            process,
            ptr::null(),
            0,
            load_library_addr,
            remote_buf,
            0,
            &mut thread_id,
        )
    };
    if thread.is_null() || thread == INVALID_HANDLE_VALUE {
        return Err(anyhow::anyhow!(
            "CreateRemoteThread: {}",
            std::io::Error::last_os_error()
        ));
    }

    // Wait for the remote thread to finish loading the DLL
    let wait_result = unsafe { WaitForSingleObject(thread, INFINITE) };
    unsafe { CloseHandle(thread) };

    if wait_result != WAIT_OBJECT_0 {
        return Err(anyhow::anyhow!(
            "WaitForSingleObject on DLL injection thread: unexpected result {}",
            wait_result
        ));
    }

    Ok(())
}

// ── BottomHalf monitoring loop ───────────────────────────────────────────────

/// Monitor the process until it exits or limits are exceeded.
pub fn bottom_half(sub: &Subprocess, d: &mut SubprocessData) -> SubprocessResult {
    let process = d.platform.process;
    let mut result = SubprocessResult::default();
    let mut running = RunningState::new();
    let quantum_ms = sub.time_quantum.as_millis() as u32;

    loop {
        if result.success_code != 0 {
            break;
        }

        let wait = unsafe { WaitForSingleObject(process, quantum_ms) };

        if wait != WAIT_TIMEOUT {
            break;
        }

        update_process_times(&d.platform, &mut result, false);
        if sub.memory_limit > 0 {
            update_process_memory(&d.platform, &mut result);
        }

        running.update(sub, &mut result);

        if let Some(ref check) = d.out_check {
            if check.check().is_err() {
                result.output_limit_exceeded = true;
                result.success_code |= EF_STDOUT_OVERFLOW;
                break;
            }
        }
        if let Some(ref check) = d.err_check {
            if check.check().is_err() {
                result.error_limit_exceeded = true;
                result.success_code |= EF_STDERR_OVERFLOW;
                break;
            }
        }
    }

    // Determine outcome
    let final_wait = unsafe { WaitForSingleObject(process, 0) };
    if final_wait == WAIT_OBJECT_0 {
        let mut exit_code: u32 = 0;
        if unsafe { GetExitCodeProcess(process, &mut exit_code) } != 0 {
            result.exit_code = exit_code;
        }
    } else {
        loop_terminate(process);
    }

    // Final measurement
    update_process_times(&d.platform, &mut result, true);
    update_process_memory(&d.platform, &mut result);

    // Cleanup handles
    unsafe {
        CloseHandle(process);
    }
    d.platform.process = INVALID_HANDLE_VALUE;

    if d.platform.job != INVALID_HANDLE_VALUE {
        unsafe {
            CloseHandle(d.platform.job);
        }
        d.platform.job = INVALID_HANDLE_VALUE;
    }

    // Post-exit limit check
    result.set_post_limits(sub);

    // Collect buffered output
    d.collect_buffers();

    {
        let buf = d.stdout_buf.lock().unwrap();
        if !buf.is_empty() {
            result.output = buf.clone();
        }
    }
    {
        let buf = d.stderr_buf.lock().unwrap();
        if !buf.is_empty() {
            result.error = buf.clone();
        }
    }

    drop(d.out_check.take());
    drop(d.err_check.take());

    result
}
