pub mod interconnect;
pub mod redirects;

// Re-export interconnect for convenience.
pub use interconnect::interconnect;

#[cfg(windows)]
pub mod platform_windows;
#[cfg(windows)]
pub use platform_windows::WindowsLoginSession;
#[cfg(unix)]
mod platform_linux;

use std::time::Duration;

use anyhow::Result;

/// Convert a Duration to microseconds.
pub fn get_micros(d: Duration) -> u64 {
    d.as_micros() as u64
}

/// Convert microseconds to a Duration.
pub fn du_from_micros(us: u64) -> Duration {
    Duration::from_micros(us)
}

// Execution result flags (bitmask).
pub const EF_INACTIVE: u32 = 1 << 0;
pub const EF_TIME_LIMIT_HIT: u32 = 1 << 1;
pub const EF_WALL_TIME_LIMIT_HIT: u32 = 1 << 2;
pub const EF_MEMORY_LIMIT_HIT: u32 = 1 << 3;
pub const EF_KILLED: u32 = 1 << 4;
pub const EF_STDOUT_OVERFLOW: u32 = 1 << 5;
pub const EF_STDERR_OVERFLOW: u32 = 1 << 6;
pub const EF_STDPIPE_TIMEOUT: u32 = 1 << 7;
pub const EF_TIME_LIMIT_HIT_POST: u32 = 1 << 8;
pub const EF_MEMORY_LIMIT_HIT_POST: u32 = 1 << 9;
pub const EF_PROCESS_LIMIT_HIT: u32 = 1 << 10;
pub const EF_PROCESS_LIMIT_HIT_POST: u32 = 1 << 11;
pub const EF_STOPPED: u32 = 1 << 12;
pub const EF_KILLED_BY_OTHER: u32 = 1 << 13;
pub const EF_WALL_TIME_LIMIT_HIT_POST: u32 = 1 << 14;
pub const EF_KERNEL_TIME_LIMIT_HIT: u32 = 1 << 15;
pub const EF_KERNEL_TIME_LIMIT_HIT_POST: u32 = 1 << 16;

/// I/O redirect mode.
#[derive(Debug, Clone, Default)]
pub enum RedirectMode {
    #[default]
    None,
    Memory,
    File,
    Pipe,
    Remote,
}

/// I/O redirect configuration.
#[derive(Debug, Default)]
pub struct Redirect {
    pub mode: RedirectMode,
    pub filename: String,
    pub data: Vec<u8>,
    pub max_output_size: i64,
    /// Pipe handle for REDIRECT_PIPE mode (set up externally, e.g. by Interconnect).
    pub pipe: Option<std::fs::File>,
}

/// Login credentials for user impersonation.
#[derive(Debug, Clone, Default)]
pub struct LoginInfo {
    pub username: String,
    pub password: String,
}

/// Command specification.
#[derive(Debug, Clone, Default)]
pub struct CommandLine {
    pub application_name: String,
    pub command_line: String,
    pub parameters: Vec<String>,
}

/// Immutable subprocess configuration.
#[derive(Debug)]
pub struct Subprocess {
    pub cmd: CommandLine,
    pub current_directory: String,
    pub environment: Vec<String>,

    pub no_inherit_environment: bool,
    pub no_job: bool,
    pub restrict_ui: bool,
    pub process_limit: u32,
    pub fail_on_job_creation_failure: bool,

    pub time_limit: Duration,
    pub kernel_time_limit: Duration,
    pub wall_time_limit: Duration,
    pub check_idleness: bool,
    pub memory_limit: u64,
    pub hard_memory_limit: u64,
    pub time_quantum: Duration,
    pub process_affinity_mask: u64,

    pub stdin: Option<Redirect>,
    pub stdout: Option<Redirect>,
    pub stderr: Option<Redirect>,
    pub join_stdout_stderr: bool,

    /// Login credentials for running the process as another user.
    pub login: Option<LoginInfo>,

    /// DLLs to inject into the process before it starts executing.
    pub inject_dll: Vec<String>,
}

impl Default for Subprocess {
    fn default() -> Self {
        Self {
            cmd: CommandLine::default(),
            current_directory: String::new(),
            environment: Vec::new(),
            no_inherit_environment: false,
            no_job: false,
            restrict_ui: false,
            process_limit: 0,
            fail_on_job_creation_failure: false,
            time_limit: Duration::ZERO,
            kernel_time_limit: Duration::ZERO,
            wall_time_limit: Duration::ZERO,
            check_idleness: false,
            memory_limit: 0,
            hard_memory_limit: 0,
            time_quantum: Duration::from_millis(250),
            process_affinity_mask: 0,
            stdin: None,
            stdout: None,
            stderr: None,
            join_stdout_stderr: false,
            login: None,
            inject_dll: Vec::new(),
        }
    }
}

/// Time statistics for a completed subprocess.
#[derive(Debug, Clone, Default)]
pub struct TimeStats {
    pub user_time: Duration,
    pub kernel_time: Duration,
    pub wall_time: Duration,
}

/// Result of a subprocess execution.
#[derive(Debug, Clone, Default)]
pub struct SubprocessResult {
    pub success_code: u32,
    pub exit_code: u32,
    pub time: TimeStats,
    pub peak_memory: u64,
    pub total_processes: u64,
    pub output_limit_exceeded: bool,
    pub error_limit_exceeded: bool,
    pub output: Vec<u8>,
    pub error: Vec<u8>,
}

impl SubprocessResult {
    /// Check limits after process exits (post-execution check).
    pub fn set_post_limits(&mut self, sub: &Subprocess) {
        if sub.time_limit > Duration::ZERO && self.time.user_time > sub.time_limit {
            self.success_code |= EF_TIME_LIMIT_HIT_POST;
        }
        if sub.memory_limit > 0 && self.peak_memory > sub.memory_limit {
            self.success_code |= EF_MEMORY_LIMIT_HIT_POST;
        }
        if sub.kernel_time_limit > Duration::ZERO && self.time.kernel_time > sub.kernel_time_limit {
            self.success_code |= EF_KERNEL_TIME_LIMIT_HIT_POST;
        }
        if sub.wall_time_limit > Duration::ZERO && self.time.wall_time > sub.wall_time_limit {
            self.success_code |= EF_WALL_TIME_LIMIT_HIT_POST;
        }
    }
}

/// Tracks running process state for idleness detection.
pub(crate) struct RunningState {
    last_time_used: Duration,
    no_time_used_count: u32,
}

impl RunningState {
    pub fn new() -> Self {
        Self {
            last_time_used: Duration::ZERO,
            no_time_used_count: 0,
        }
    }

    /// Update state and check soft limits. Sets flags in result.success_code.
    pub fn update(&mut self, sub: &Subprocess, result: &mut SubprocessResult) {
        let total = result.time.kernel_time + result.time.user_time;

        if total == self.last_time_used {
            self.no_time_used_count += 1;
        } else {
            self.no_time_used_count = 0;
        }

        if sub.check_idleness
            && self.no_time_used_count >= 6
            && result.time.wall_time > sub.time_limit
        {
            result.success_code |= EF_INACTIVE;
        }

        if sub.time_limit > Duration::ZERO && result.time.user_time > sub.time_limit {
            result.success_code |= EF_TIME_LIMIT_HIT;
        }

        if sub.kernel_time_limit > Duration::ZERO
            && result.time.kernel_time > sub.kernel_time_limit
        {
            result.success_code |= EF_KERNEL_TIME_LIMIT_HIT;
        }

        if sub.wall_time_limit > Duration::ZERO && result.time.wall_time > sub.wall_time_limit {
            result.success_code |= EF_WALL_TIME_LIMIT_HIT;
        }

        self.last_time_used = total;

        if sub.memory_limit > 0 && result.peak_memory > sub.memory_limit {
            result.success_code |= EF_MEMORY_LIMIT_HIT;
        }
    }
}

/// Check if an error is a user error (as opposed to system error).
pub fn is_user_error(_err: &anyhow::Error) -> bool {
    // TODO: implement proper error classification
    false
}

// ── Execute ──────────────────────────────────────────────────────────────────

impl Subprocess {
    /// Execute the subprocess: create frozen, inject DLLs, set up buffers, unfreeze, monitor.
    #[cfg(windows)]
    pub fn execute(&self) -> Result<SubprocessResult> {
        let mut d = platform_windows::create_frozen(self).map_err(|e| {
            // cleanup_if_failed is already handled inside create_frozen on error
            e
        })?;

        // Inject DLLs while the process is still suspended.
        // Resolve the correct LoadLibraryW address once (handles 32-bit targets on 64-bit host).
        if !self.inject_dll.is_empty() {
            let load_library_addr =
                platform_windows::resolve_load_library_for_target(&self.cmd)
                    .map_err(|e| {
                        platform_windows::terminate_frozen(&mut d);
                        e.context("resolving LoadLibraryW for DLL injection")
                    })?;

            for dll in &self.inject_dll {
                if let Err(e) = platform_windows::inject_dll(&d, dll, load_library_addr) {
                    platform_windows::terminate_frozen(&mut d);
                    return Err(e.context(format!("InjectDll({:?})", dll)));
                }
            }
        }

        d.setup_redirection_buffers();
        platform_windows::unfreeze(&mut d);
        Ok(platform_windows::bottom_half(self, &mut d))
    }

    #[cfg(unix)]
    pub fn execute(&self) -> Result<SubprocessResult> {
        anyhow::bail!("subprocess execution not implemented on this platform")
    }
}
