//! Interactive pipe plumbing and interaction log.
//!
//! Provides `Interconnect` to cross-connect stdin/stdout of two subprocesses,
//! and `InteractionLog` to record the data flow with direction markers.

use std::fs::File;
use std::io::{self, BufWriter, Write};
use std::sync::{Arc, Mutex};

use anyhow::{Context, Result};

#[cfg(windows)]
use std::os::windows::io::{FromRawHandle, IntoRawHandle};

use crate::{Redirect, RedirectMode, Subprocess};

/// Create a pair of pipes suitable for inheriting into child processes.
/// On Windows, uses os_pipe which creates inheritable handles.
fn hack_pipe() -> Result<(File, File)> {
    let (reader, writer) = os_pipe::pipe().context("creating pipe")?;
    let reader_file: File = unsafe { File::from_raw_handle(reader.into_raw_handle()) };
    let writer_file: File = unsafe { File::from_raw_handle(writer.into_raw_handle()) };
    Ok((reader_file, writer_file))
}

/// Records a data transfer event.
#[derive(Debug, Clone)]
pub struct PipeRecordEntry {
    pub direction: i32,
    pub bytes: i64,
    pub error: Option<String>,
}

/// Thread-safe recorder for pipe transfer events.
pub struct OrderedRecorder {
    entries: Mutex<Vec<PipeRecordEntry>>,
}

impl OrderedRecorder {
    pub fn new() -> Self {
        Self {
            entries: Mutex::new(Vec::new()),
        }
    }

    pub fn record(&self, direction: i32, num_bytes: i64, err: Option<String>) {
        self.entries.lock().unwrap().push(PipeRecordEntry {
            direction,
            bytes: num_bytes,
            error: err,
        });
    }

    pub fn get_entries(&self) -> Vec<PipeRecordEntry> {
        self.entries.lock().unwrap().clone()
    }
}

/// Interaction log writer that prefixes lines with direction markers.
/// Format: "< " for direction 0, "> " for direction 1, "\n~" for direction switch mid-line.
pub struct InteractionLog {
    writer: BufWriter<File>,
    had_eol: bool,
    current_line_direction: i32,
}

impl InteractionLog {
    pub fn new(file: File) -> Self {
        Self {
            writer: BufWriter::new(file),
            had_eol: true,
            current_line_direction: -1,
        }
    }

    pub fn write_data(&mut self, direction: i32, p: &[u8]) -> io::Result<usize> {
        if p.is_empty() {
            return Ok(0);
        }

        let switch_without_eol = b"\n~";
        let prefix: &[u8] = if direction == 0 { b"< " } else { b"> " };

        let mut n = 0usize;
        let lines: Vec<&[u8]> = p.split(|&b| b == b'\n').collect();

        for (i, line) in lines.iter().enumerate() {
            // If this is the last empty segment after a trailing \n, skip it
            if i + 1 >= lines.len() && line.is_empty() {
                break;
            }

            if direction != self.current_line_direction {
                if !self.had_eol {
                    self.writer.write_all(switch_without_eol)?;
                }
                self.current_line_direction = direction;
                self.writer.write_all(prefix)?;
            }

            self.writer.write_all(line)?;
            n += line.len();

            if i + 1 < lines.len() {
                self.writer.write_all(b"\n")?;
                n += 1;
                self.had_eol = true;
                self.current_line_direction = -1;
            } else {
                self.had_eol = false;
            }
        }

        Ok(n)
    }

    pub fn flush(&mut self) -> io::Result<()> {
        self.writer.flush()
    }
}

/// Copy data from `reader` to `writer`, optionally recording to a tap writer
/// and an interaction log.
fn recording_tee(
    mut writer: File,
    mut reader: File,
    tap: Option<File>,
    interaction_log: Option<Arc<Mutex<InteractionLog>>>,
    direction: i32,
    recorder: Option<Arc<OrderedRecorder>>,
) {
    let mut buf = [0u8; 8192];
    let mut total: i64 = 0;
    let mut err_msg = None;

    loop {
        match io::Read::read(&mut reader, &mut buf) {
            Ok(0) => break, // EOF
            Ok(n) => {
                total += n as i64;
                // Write to tap file (recording)
                if let Some(ref mut tap) = tap.as_ref() {
                    let _ = io::Write::write_all(&mut &**tap, &buf[..n]);
                }
                // Write to interaction log
                if let Some(ref ilog) = interaction_log {
                    let _ = ilog.lock().unwrap().write_data(direction, &buf[..n]);
                }
                // Write to actual destination
                if let Err(e) = io::Write::write_all(&mut writer, &buf[..n]) {
                    err_msg = Some(e.to_string());
                    break;
                }
            }
            Err(e) => {
                err_msg = Some(e.to_string());
                break;
            }
        }
    }

    if let Some(ref rec) = recorder {
        rec.record(direction, total, err_msg);
    }
}

/// A recording pipe: reads from one end, writes to the other, optionally
/// teeing data to a recorder and interaction log.
/// Returns (read_end, write_end) — the read_end goes to the consumer process,
/// the write_end goes to the producer process.
fn recording_pipe(
    tap: Option<File>,
    interaction_log: Option<Arc<Mutex<InteractionLog>>>,
    direction: i32,
    recorder: Option<Arc<OrderedRecorder>>,
) -> Result<(File, File)> {
    if tap.is_none() && recorder.is_none() && interaction_log.is_none() {
        return hack_pipe();
    }

    let (r1, w1) = hack_pipe().context("first pipe")?;
    let (r2, w2) = hack_pipe().context("second pipe")?;

    std::thread::spawn(move || {
        recording_tee(w1, r2, tap, interaction_log, direction, recorder);
    });

    Ok((r1, w2))
}

/// Cross-connect stdin/stdout of two subprocesses for interactive mode.
///
/// After this call:
/// - s1.stdin reads from s2.stdout (through a pipe)
/// - s2.stdin reads from s1.stdout (through a pipe)
///
/// Optional recording files capture the data flowing through each pipe.
pub fn interconnect(
    s1: &mut Subprocess,
    s2: &mut Subprocess,
    record_input: Option<File>,
    record_output: Option<File>,
    interaction_log_file: Option<File>,
    recorder: Option<Arc<OrderedRecorder>>,
) -> Result<()> {
    let ilog = interaction_log_file.map(|f| Arc::new(Mutex::new(InteractionLog::new(f))));

    // Pipe 1: s2.stdout → s1.stdin (program reads interactor output)
    // Direction 0: data flowing from interactor to program
    let (read1, write1) = recording_pipe(
        record_input,
        ilog.clone(),
        0,
        recorder.clone(),
    )?;

    // Pipe 2: s1.stdout → s2.stdin (interactor reads program output)
    // Direction 1: data flowing from program to interactor
    let (read2, write2) = recording_pipe(
        record_output,
        ilog,
        1,
        recorder,
    )?;

    s1.stdin = Some(Redirect {
        mode: RedirectMode::Pipe,
        pipe: Some(read1),
        ..Default::default()
    });
    s2.stdout = Some(Redirect {
        mode: RedirectMode::Pipe,
        pipe: Some(write1),
        ..Default::default()
    });
    s1.stdout = Some(Redirect {
        mode: RedirectMode::Pipe,
        pipe: Some(write2),
        ..Default::default()
    });
    s2.stdin = Some(Redirect {
        mode: RedirectMode::Pipe,
        pipe: Some(read2),
        ..Default::default()
    });

    Ok(())
}
