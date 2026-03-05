//! CLI flag parsing for runexe, matching the Go runexe flag scheme.

use std::fmt;

use clap::Parser;

/// Parse a time limit value: supports "1s", "500ms", bare number (seconds as float).
/// Internally stored as microseconds.
fn parse_time_limit(s: &str) -> Result<u64, String> {
    let s = s.to_lowercase();
    if let Some(rest) = s.strip_suffix("ms") {
        let ms: f64 = rest.parse().map_err(|e| format!("invalid time: {e}"))?;
        if ms < 0.0 {
            return Err(format!("invalid time limit {s}"));
        }
        Ok((ms * 1000.0) as u64)
    } else {
        let rest = s.strip_suffix('s').unwrap_or(&s);
        let secs: f64 = rest.parse().map_err(|e| format!("invalid time: {e}"))?;
        if secs < 0.0 {
            return Err(format!("invalid time limit {s}"));
        }
        Ok((secs * 1_000_000.0) as u64)
    }
}

/// Parse a memory limit value: supports suffixes K, M, G.
fn parse_memory_limit(s: &str) -> Result<u64, String> {
    let s_upper = s.to_uppercase();
    let (num_str, multiplier) = match s_upper.as_bytes().last() {
        Some(b'K') => (&s_upper[..s_upper.len() - 1], 1024u64),
        Some(b'M') => (&s_upper[..s_upper.len() - 1], 1024 * 1024),
        Some(b'G') => (&s_upper[..s_upper.len() - 1], 1024 * 1024 * 1024),
        _ => (s_upper.as_str(), 1),
    };
    let val: u64 = num_str.parse().map_err(|e| format!("invalid memory: {e}"))?;
    Ok(val * multiplier)
}

/// Parse a process affinity mask: if starts with '0', parse as binary; else decimal.
fn parse_affinity(s: &str) -> Result<u64, String> {
    if s.is_empty() {
        return Ok(0);
    }
    if s.starts_with('0') && s.len() > 1 {
        u64::from_str_radix(&s[1..], 2).map_err(|e| format!("invalid affinity: {e}"))
    } else {
        s.parse().map_err(|e| format!("invalid affinity: {e}"))
    }
}

/// Process configuration flags.
#[derive(Parser, Debug)]
#[command(name = "runexe", version, about = "Run a program with resource limits", disable_help_flag = true)]
pub struct RunexeArgs {
    /// Show help information
    #[arg(long = "help", action = clap::ArgAction::Help)]
    pub help: Option<bool>,
    // ── Global options ──

    /// Print result in XML format
    #[arg(long = "xml")]
    pub xml: bool,

    /// Interactor command line
    #[arg(long = "interactor")]
    pub interactor: Option<String>,

    /// Show kernel-mode time in text output
    #[arg(long = "show-kernel-mode-time")]
    pub show_kernel_mode_time: bool,

    /// Return process exit code as runexe exit code
    #[arg(short = 'x')]
    pub return_exit_code: bool,

    /// Log file for runexe developers
    #[arg(long = "logfile")]
    pub logfile: Option<String>,

    /// Record program input to file (interactor mode)
    #[arg(long = "ri")]
    pub record_program_input: Option<String>,

    /// Record program output to file (interactor mode)
    #[arg(long = "ro")]
    pub record_program_output: Option<String>,

    /// Record interaction log to file (interactor mode)
    #[arg(long = "interaction-log")]
    pub record_interaction_log: Option<String>,

    // ── Process properties ──

    /// Time limit (e.g. "1s", "500ms", "2.5")
    #[arg(short = 't', value_parser = parse_time_limit)]
    pub time_limit: Option<u64>,

    /// Wall time limit
    #[arg(short = 'h', value_parser = parse_time_limit)]
    pub wall_time_limit: Option<u64>,

    /// Memory limit (e.g. "256M", "1G", raw bytes)
    #[arg(short = 'm', value_parser = parse_memory_limit)]
    pub memory_limit: Option<u64>,

    /// Environment variables (clears existing env)
    #[arg(short = 'D')]
    pub environment: Vec<String>,

    /// Environment file (clears existing env)
    #[arg(long = "envfile")]
    pub environment_file: Option<String>,

    /// Current directory for the process
    #[arg(short = 'd')]
    pub current_directory: Option<String>,

    /// Login name (create process under this user)
    #[arg(short = 'l')]
    pub login_name: Option<String>,

    /// Password for user specified in -l
    #[arg(short = 'p')]
    pub password: Option<String>,

    /// DLL to inject into process
    #[arg(short = 'j')]
    pub inject_dll: Option<String>,

    /// Redirect stdin from file
    #[arg(short = 'i')]
    pub stdin_file: Option<String>,

    /// Redirect stdout to file
    #[arg(short = 'o')]
    pub stdout_file: Option<String>,

    /// Redirect stderr to file
    #[arg(short = 'e')]
    pub stderr_file: Option<String>,

    /// Max stdout file size
    #[arg(long = "os")]
    pub stdout_max_size: Option<i64>,

    /// Max stderr file size
    #[arg(long = "es")]
    pub stderr_max_size: Option<i64>,

    /// Join stderr to stdout
    #[arg(short = 'u')]
    pub join_stdout_stderr: bool,

    /// Don't restrict process UI (trusted mode)
    #[arg(short = 'z')]
    pub trusted_mode: bool,

    /// Switch off idleness checking
    #[arg(long = "no-idleness-check")]
    pub no_idle_check: bool,

    /// Don't create job objects
    #[arg(long = "no-job")]
    pub no_job: bool,

    /// Process limit (requires job object)
    #[arg(long = "process-limit")]
    pub process_limit: Option<u32>,

    /// Process affinity (decimal or 0-prefixed binary)
    #[arg(short = 'a', value_parser = parse_affinity)]
    pub process_affinity: Option<u64>,

    /// Program and its arguments
    #[arg(trailing_var_arg = true, required = true)]
    pub program: Vec<String>,
}

/// Verdict for a subprocess execution.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Verdict {
    Success,
    Fail,
    Crash,
    TimeLimitExceeded,
    MemoryLimitExceeded,
    Idle,
    SecurityViolation,
    OutputLimitExceeded,
}

impl fmt::Display for Verdict {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Success => write!(f, "SUCCEEDED"),
            Self::Fail => write!(f, "FAILED"),
            Self::Crash => write!(f, "CRASHED"),
            Self::TimeLimitExceeded => write!(f, "TIME_LIMIT_EXCEEDED"),
            Self::MemoryLimitExceeded => write!(f, "MEMORY_LIMIT_EXCEEDED"),
            Self::Idle => write!(f, "IDLENESS_LIMIT_EXCEEDED"),
            Self::SecurityViolation => write!(f, "SECURITY_VIOLATION"),
            Self::OutputLimitExceeded => write!(f, "OUTPUT_LIMIT_EXCEEDED"),
        }
    }
}

/// Process type identifier.
#[derive(Debug, Clone, Copy)]
pub enum ProcessType {
    Program,
    Interactor,
}

impl fmt::Display for ProcessType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Program => write!(f, "Program"),
            Self::Interactor => write!(f, "Interactor"),
        }
    }
}

impl ProcessType {
    pub fn xml_id(&self) -> &'static str {
        match self {
            Self::Program => "program",
            Self::Interactor => "interactor",
        }
    }
}
