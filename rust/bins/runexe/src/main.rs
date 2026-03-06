//! runexe — CLI tool for running a program with resource limits.

mod flags;
mod results;

use std::io::BufRead;
use std::sync::Arc;
use std::thread;

use anyhow::{bail, Result};
use clap::Parser;

use contester_subprocess::{
    LoginInfo, Redirect, RedirectMode, Subprocess,
    du_from_micros, is_user_error,
};
use contester_subprocess::interconnect::OrderedRecorder;

use flags::{ProcessType, RunexeArgs, Verdict};
use results::{
    RunResult, fail_text, fail_xml, get_verdict,
    print_result_text, print_results_xml, XML_HEADER,
};

/// Build a subprocess configuration from CLI flags.
fn setup_subprocess(args: &RunexeArgs) -> Result<Subprocess> {
    let mut sub = Subprocess::default();

    // Command line — on Windows, escape args for CreateProcessW
    #[cfg(windows)]
    {
        let escaped: Vec<String> = args.program.iter().map(|a| escape_arg(a)).collect();
        sub.cmd.command_line = escaped.join(" ");
    }
    #[cfg(not(windows))]
    {
        if !args.program.is_empty() {
            sub.cmd.application_name = args.program[0].clone();
            sub.cmd.command_line = args.program.join(" ");
        }
    }
    sub.cmd.parameters = args.program.clone();

    // Current directory
    if let Some(ref d) = args.current_directory {
        sub.current_directory = d.clone();
    } else if let Ok(cwd) = std::env::current_dir() {
        sub.current_directory = cwd.to_string_lossy().into_owned();
    }

    // Limits
    if let Some(t) = args.time_limit {
        sub.time_limit = du_from_micros(t);
    }
    if let Some(h) = args.wall_time_limit {
        sub.wall_time_limit = du_from_micros(h);
    }
    if let Some(m) = args.memory_limit {
        sub.memory_limit = m;
    }

    sub.check_idleness = !args.no_idle_check;
    sub.restrict_ui = !args.trusted_mode;
    sub.no_job = args.no_job;

    if let Some(a) = args.process_affinity {
        sub.process_affinity_mask = a;
    }

    if let Some(pl) = args.process_limit {
        if args.no_job {
            bail!("can't enforce process limit if not using job object");
        }
        sub.fail_on_job_creation_failure = true;
        sub.process_limit = pl;
    }

    // Environment
    if let Some(ref envfile) = args.environment_file {
        sub.environment = read_environment_file(envfile)?;
        sub.no_inherit_environment = true;
    } else if !args.environment.is_empty() {
        sub.environment = args.environment.clone();
        sub.no_inherit_environment = true;
    }

    // Redirects
    sub.stdin = fill_redirect(args.stdin_file.as_deref(), 0);
    sub.stdout = fill_redirect(args.stdout_file.as_deref(), args.stdout_max_size.unwrap_or(0));
    if args.join_stdout_stderr {
        sub.join_stdout_stderr = true;
    } else {
        sub.stderr = fill_redirect(args.stderr_file.as_deref(), args.stderr_max_size.unwrap_or(0));
    }

    // Login (user impersonation)
    if let Some(ref login_name) = args.login_name {
        sub.login = Some(LoginInfo {
            username: login_name.clone(),
            password: args.password.as_deref().unwrap_or("").to_string(),
        });
    }

    // DLL injection
    if let Some(ref dll) = args.inject_dll {
        sub.inject_dll.push(dll.clone());
    }

    Ok(sub)
}

fn fill_redirect(filename: Option<&str>, max_size: i64) -> Option<Redirect> {
    let name = filename?;
    if name.is_empty() {
        return None;
    }
    Some(Redirect {
        filename: name.to_string(),
        mode: RedirectMode::File,
        max_output_size: max_size,
        ..Default::default()
    })
}

fn read_environment_file(name: &str) -> Result<Vec<String>> {
    let file = std::fs::File::open(name)?;
    let reader = std::io::BufReader::new(file);
    let mut result = Vec::new();
    for line in reader.lines() {
        result.push(line?);
    }
    Ok(result)
}

/// Execute a subprocess and produce a RunResult.
fn exec_and_result(sub: Subprocess, ptype: ProcessType) -> RunResult {
    match sub.execute() {
        Ok(r) => {
            let v = get_verdict(&r);
            RunResult {
                verdict: v,
                error: None,
                subprocess: sub,
                result: Some(r),
                process_type: ptype,
            }
        }
        Err(e) => {
            let v = if is_user_error(&e) {
                Verdict::Crash
            } else {
                Verdict::Fail
            };
            RunResult {
                verdict: v,
                error: Some(e),
                subprocess: sub,
                result: None,
                process_type: ptype,
            }
        }
    }
}

/// Parse a command line string into arguments, following Windows conventions.
/// Port of Go's commandLineToArgv.
fn command_line_to_argv(cmd: &str) -> Vec<String> {
    let mut args = Vec::new();
    let mut rest = cmd;
    while !rest.is_empty() {
        let first = rest.as_bytes()[0];
        if first == b' ' || first == b'\t' {
            rest = &rest[1..];
            continue;
        }
        let (arg, remainder) = read_next_arg(rest);
        args.push(String::from_utf8_lossy(&arg).into_owned());
        rest = remainder;
    }
    args
}

fn read_next_arg(cmd: &str) -> (Vec<u8>, &str) {
    let mut b = Vec::new();
    let mut inquote = false;
    let mut nslash = 0usize;
    let bytes = cmd.as_bytes();
    let mut i = 0;

    while i < bytes.len() {
        let c = bytes[i];
        match c {
            b' ' | b'\t' if !inquote => {
                append_bs_bytes(&mut b, nslash);
                return (b, &cmd[i + 1..]);
            }
            b'"' => {
                append_bs_bytes(&mut b, nslash / 2);
                if nslash % 2 == 0 {
                    if inquote && i + 1 < bytes.len() && bytes[i + 1] == b'"' {
                        b.push(c);
                        i += 1;
                    }
                    inquote = !inquote;
                } else {
                    b.push(c);
                }
                nslash = 0;
                i += 1;
                continue;
            }
            b'\\' => {
                nslash += 1;
                i += 1;
                continue;
            }
            _ => {}
        }
        append_bs_bytes(&mut b, nslash);
        nslash = 0;
        b.push(c);
        i += 1;
    }
    append_bs_bytes(&mut b, nslash);
    (b, "")
}

fn append_bs_bytes(b: &mut Vec<u8>, n: usize) {
    for _ in 0..n {
        b.push(b'\\');
    }
}

/// Escape a single argument for Windows command line (matching Go's syscall.EscapeArg).
#[cfg(windows)]
fn escape_arg(s: &str) -> String {
    if s.is_empty() {
        return "\"\"".to_string();
    }
    let needs_escape = s.bytes().any(|b| b == b' ' || b == b'\t' || b == b'"');
    if !needs_escape {
        return s.to_string();
    }

    let mut result = Vec::new();
    result.push(b'"');
    let bytes = s.as_bytes();
    let mut i = 0;
    while i < bytes.len() {
        let c = bytes[i];
        if c == b'\\' {
            let mut j = i;
            while j < bytes.len() && bytes[j] == b'\\' {
                j += 1;
            }
            let nslash = j - i;
            if j == bytes.len() {
                // Trailing backslashes — double them
                for _ in 0..nslash * 2 {
                    result.push(b'\\');
                }
                i = j;
            } else if bytes[j] == b'"' {
                // Backslashes before quote — double + escape quote
                for _ in 0..nslash * 2 {
                    result.push(b'\\');
                }
                result.push(b'\\');
                result.push(b'"');
                i = j + 1;
            } else {
                for _ in 0..nslash {
                    result.push(b'\\');
                }
                i = j;
            }
        } else if c == b'"' {
            result.push(b'\\');
            result.push(b'"');
            i += 1;
        } else {
            result.push(c);
            i += 1;
        }
    }
    result.push(b'"');
    String::from_utf8(result).unwrap_or_else(|_| s.to_string())
}

/// Parse interactor flags from the interactor command string.
fn parse_interactor_args(interactor_cmd: &str) -> Result<RunexeArgs> {
    let argv = command_line_to_argv(interactor_cmd);
    // Prepend a dummy program name for clap
    let mut full_argv = vec!["runexe".to_string()];
    full_argv.extend(argv);
    let args = RunexeArgs::try_parse_from(full_argv)?;
    Ok(args)
}

fn main() {
    let args = RunexeArgs::parse();

    // Set up logging
    if let Some(ref logfile) = args.logfile {
        let logfile = logfile.clone();
        let _ = tracing_subscriber::fmt()
            .with_writer(move || {
                std::fs::OpenOptions::new()
                    .create(true)
                    .append(true)
                    .open(&logfile)
                    .unwrap()
            })
            .try_init();
    }

    let use_xml = args.xml;

    let fail: fn(&dyn std::fmt::Display, &str) = if use_xml {
        fail_xml
    } else {
        fail_text
    };

    if use_xml {
        println!("{XML_HEADER}");
    }

    // Set up main program
    let mut program = match setup_subprocess(&args) {
        Ok(s) => s,
        Err(e) => {
            fail(&e, "Setup main subprocess");
            std::process::exit(3);
        }
    };

    let recorder = if !use_xml {
        Some(Arc::new(OrderedRecorder::new()))
    } else {
        None
    };

    // Set up interactor if specified
    let mut interactor = if let Some(ref interactor_cmd) = args.interactor {
        let iargs = match parse_interactor_args(interactor_cmd) {
            Ok(a) => a,
            Err(e) => {
                fail(&e, "Parse interactor flags");
                std::process::exit(3);
            }
        };
        match setup_subprocess(&iargs) {
            Ok(s) => Some(s),
            Err(e) => {
                fail(&e, "Setup interactor subprocess");
                std::process::exit(3);
            }
        }
    } else {
        None
    };

    // Interconnect if interactor mode
    if interactor.is_some() {
        let record_input = args
            .record_program_input
            .as_ref()
            .and_then(|f| std::fs::File::create(f).ok());
        let record_output = args
            .record_program_output
            .as_ref()
            .and_then(|f| std::fs::File::create(f).ok());
        let interaction_log = args
            .record_interaction_log
            .as_ref()
            .and_then(|f| std::fs::File::create(f).ok());

        if let Err(e) = contester_subprocess::interconnect::interconnect(
            &mut program,
            interactor.as_mut().unwrap(),
            record_input,
            record_output,
            interaction_log,
            recorder.clone(),
        ) {
            fail(&e, "Interconnect");
            std::process::exit(3);
        }
    }

    // Execute
    let mut results: [Option<RunResult>; 2] = [None, None];

    if let Some(interactor_sub) = interactor {
        // Run both in parallel
        let handle_interactor = thread::spawn(move || {
            exec_and_result(interactor_sub, ProcessType::Interactor)
        });

        results[0] = Some(exec_and_result(program, ProcessType::Program));
        results[1] = Some(handle_interactor.join().unwrap());
    } else {
        results[0] = Some(exec_and_result(program, ProcessType::Program));
    }

    // Get program exit code
    let program_return_code = results[0]
        .as_ref()
        .and_then(|r| r.result.as_ref())
        .map(|r| r.exit_code as i32)
        .unwrap_or(0);

    // Output results
    if use_xml {
        print_results_xml(&results);
    } else {
        for result in results.iter().flatten() {
            print_result_text(args.show_kernel_mode_time, result);
        }
    }

    if args.return_exit_code {
        std::process::exit(program_return_code);
    }
}
