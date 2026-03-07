//! Verdict determination and output formatting (text and XML).

use std::time::Duration;

use contester_subprocess::{ExecutionFlags, SubprocessResult, Subprocess};
use quick_xml::se::to_string as xml_to_string;
use serde::Serialize;

use crate::flags::{ProcessType, Verdict};

/// Determine the verdict from a SubprocessResult.
pub fn get_verdict(r: &SubprocessResult) -> Verdict {
    if r.output_limit_exceeded || r.error_limit_exceeded {
        Verdict::OutputLimitExceeded
    } else if r.success_code.is_empty() {
        Verdict::Success
    } else if r.success_code.intersects(ExecutionFlags::PROCESS_LIMIT_HIT | ExecutionFlags::PROCESS_LIMIT_HIT_POST) {
        Verdict::SecurityViolation
    } else if r.success_code.intersects(ExecutionFlags::INACTIVE | ExecutionFlags::WALL_TIME_LIMIT_HIT) {
        Verdict::Idle
    } else if r.success_code.intersects(
        ExecutionFlags::TIME_LIMIT_HIT | ExecutionFlags::TIME_LIMIT_HIT_POST
            | ExecutionFlags::KERNEL_TIME_LIMIT_HIT | ExecutionFlags::KERNEL_TIME_LIMIT_HIT_POST,
    ) {
        Verdict::TimeLimitExceeded
    } else if r.success_code.intersects(ExecutionFlags::MEMORY_LIMIT_HIT | ExecutionFlags::MEMORY_LIMIT_HIT_POST) {
        Verdict::MemoryLimitExceeded
    } else {
        Verdict::Crash
    }
}

/// Result of running a single process.
pub struct RunResult {
    pub verdict: Verdict,
    pub error: Option<anyhow::Error>,
    pub subprocess: Subprocess,
    pub result: Option<SubprocessResult>,
    pub process_type: ProcessType,
}

// ── Text output ──

fn str_time(d: Duration) -> String {
    format!("{:.2}", d.as_secs_f64())
}

fn str_memory(m: u64) -> String {
    m.to_string()
}

pub fn print_result_text(kernel_time: bool, result: &RunResult) {
    let ptype = &result.process_type;

    match result.verdict {
        Verdict::Success => {
            let r = result.result.as_ref().unwrap();
            println!("{ptype} successfully terminated");
            println!("  exit code:    {}", r.exit_code);
        }
        Verdict::OutputLimitExceeded => {
            println!("{ptype} output limit exceeded");
        }
        Verdict::TimeLimitExceeded => {
            println!("Time limit exceeded");
            println!(
                "{ptype} failed to terminate within {} sec",
                str_time(result.subprocess.time_limit)
            );
        }
        Verdict::MemoryLimitExceeded => {
            println!("Memory limit exceeded");
            println!(
                "{ptype} tried to allocate more than {} bytes",
                str_memory(result.subprocess.memory_limit)
            );
        }
        Verdict::Idle => {
            println!("Idleness limit exceeded");
            println!("Detected {ptype} idle");
        }
        Verdict::SecurityViolation => {
            println!("Security violation");
            println!("{ptype} tried to do some forbidden action");
        }
        Verdict::Crash => {
            println!("Invocation crashed: {ptype}");
            if let Some(ref e) = result.error {
                println!("Comment: {e}");
            }
            println!();
            return;
        }
        Verdict::Fail => {
            println!("Invocation failed: {ptype}");
            if let Some(ref e) = result.error {
                println!("Comment: {e}");
            }
            println!();
            return;
        }
    }

    let r = result.result.as_ref().unwrap();
    let usuffix = if result.verdict == Verdict::TimeLimitExceeded {
        format!("of {} sec", str_time(result.subprocess.time_limit))
    } else {
        "sec".to_string()
    };

    let utime = format!("{} {usuffix}", str_time(r.time.user_time));
    if kernel_time {
        println!("  time consumed:");
        println!("    user mode:   {utime}");
        println!("    kernel mode: {} sec", str_time(r.time.kernel_time));
    } else {
        println!("  time consumed: {utime}");
    }
    println!("  time passed:  {} sec", str_time(r.time.wall_time));
    println!("  peak memory:  {} bytes", str_memory(r.peak_memory));
    println!();
}

// ── XML output ──

pub const XML_HEADER: &str = "<?xml version=\"1.1\" encoding=\"UTF-8\"?>";

#[derive(Serialize)]
#[serde(rename = "invocationResult")]
struct InvocationSuccess {
    #[serde(rename = "@id")]
    id: String,
    #[serde(rename = "invocationVerdict")]
    verdict: String,
    #[serde(rename = "exitCode")]
    exit_code: i32,
    #[serde(rename = "processorUserModeTime")]
    user_time: i64,
    #[serde(rename = "processorKernelModeTime")]
    kernel_time: i64,
    #[serde(rename = "passedTime")]
    wall_time: i64,
    #[serde(rename = "consumedMemory")]
    memory: i64,
}

#[derive(Serialize)]
#[serde(rename = "invocationResult")]
struct InvocationError {
    #[serde(rename = "@id")]
    id: String,
    #[serde(rename = "invocationVerdict")]
    verdict: String,
    #[serde(rename = "comment")]
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
}

#[derive(Serialize)]
#[serde(rename = "invocationResults")]
struct InvocationResults {
    #[serde(rename = "$value")]
    results: Vec<InvocationResultEnum>,
}

#[derive(Serialize)]
#[serde(untagged)]
enum InvocationResultEnum {
    Success(InvocationSuccess),
    Error(InvocationError),
}

pub fn print_results_xml(results: &[Option<RunResult>]) {
    let mut xml_results = Vec::new();

    for result in results.iter().flatten() {
        if let Some(ref r) = result.result {
            xml_results.push(InvocationResultEnum::Success(InvocationSuccess {
                id: result.process_type.xml_id().to_string(),
                verdict: result.verdict.to_string(),
                exit_code: r.exit_code as i32,
                user_time: r.time.user_time.as_millis() as i64,
                kernel_time: r.time.kernel_time.as_millis() as i64,
                wall_time: r.time.wall_time.as_millis() as i64,
                memory: r.peak_memory as i64,
            }));
        } else {
            xml_results.push(InvocationResultEnum::Error(InvocationError {
                id: result.process_type.xml_id().to_string(),
                verdict: result.verdict.to_string(),
                error: result.error.as_ref().map(|e| e.to_string()),
            }));
        }
    }

    let wrapper = InvocationResults {
        results: xml_results,
    };

    match xml_to_string(&wrapper) {
        Ok(s) => println!("{s}"),
        Err(e) => eprintln!("XML serialization error: {e}"),
    }
}

pub fn fail_text(err: &dyn std::fmt::Display, state: &str) {
    println!("Invocation failed");
    println!("Comment: ({state}) {err}");
    println!();
    println!("Use \"runexe -h\" to get help information");
}

pub fn fail_xml(err: &dyn std::fmt::Display, state: &str) {
    let wrapper = InvocationResults {
        results: vec![InvocationResultEnum::Error(InvocationError {
            id: "program".to_string(),
            verdict: Verdict::Fail.to_string(),
            error: Some(format!("({state}) {err}")),
        })],
    };
    if let Ok(s) = xml_to_string(&wrapper) {
        println!("{s}");
    }
}
