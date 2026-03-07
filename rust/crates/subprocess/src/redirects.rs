//! I/O redirect setup for subprocess stdin/stdout/stderr.
//!
//! Provides functions to set up file, memory, and pipe-based redirects.

use std::fs::{File, OpenOptions};
use std::io::{Read, Write};
use std::sync::mpsc;

#[cfg(windows)]
use std::os::windows::fs::OpenOptionsExt;
#[cfg(windows)]
use std::os::windows::io::{FromRawHandle, IntoRawHandle};

use anyhow::{Context, Result};

use crate::{Redirect, RedirectMode};

/// Maximum output size when none is specified (1 GiB).
pub const MAX_MEM_OUTPUT: i64 = 1024 * 1024 * 1024;

/// Checks the size of an output file against a maximum.
pub struct OutputRedirectCheck {
    pub name: String,
    pub file: File,
    pub max_size: i64,
}

impl OutputRedirectCheck {
    /// Check if the output file has exceeded the size limit.
    pub fn check(&self) -> Result<()> {
        let meta = self.file.metadata()?;
        if meta.len() as i64 > self.max_size {
            anyhow::bail!("{:?}: output size {} exceeded", self.name, self.max_size);
        }
        Ok(())
    }
}

/// A function to run after process creation (in a background thread).
type BufferTask = Box<dyn FnOnce() -> Result<()> + Send + 'static>;

/// Mutable state for a running subprocess — redirect handles, buffer tasks, cleanup.
pub struct SubprocessData {
    /// Background tasks to launch after CreateFrozen (e.g. pipe copy threads).
    pub start_after_start: Vec<BufferTask>,
    /// Files to close after CreateFrozen (parent-side pipe ends).
    pub close_after_start: Vec<File>,
    /// Cleanup functions to run if CreateFrozen fails.
    pub cleanup_if_failed: Vec<Box<dyn FnOnce() + Send>>,

    /// Output size checkers for stdout/stderr file redirects.
    pub out_check: Option<OutputRedirectCheck>,
    pub err_check: Option<OutputRedirectCheck>,

    /// Buffers for memory-mode output capture (shared with background threads).
    pub stdout_buf: std::sync::Arc<std::sync::Mutex<Vec<u8>>>,
    pub stderr_buf: std::sync::Arc<std::sync::Mutex<Vec<u8>>>,

    /// Number of buffer tasks launched.
    pub buffer_count: usize,
    /// Channel to receive buffer task completion results.
    pub buffer_rx: Option<mpsc::Receiver<Result<()>>>,

    /// Platform-specific data (handles etc).
    #[cfg(windows)]
    pub platform: super::platform_windows::PlatformData,
}

impl SubprocessData {
    pub fn new() -> Self {
        Self {
            start_after_start: Vec::new(),
            close_after_start: Vec::new(),
            cleanup_if_failed: Vec::new(),
            out_check: None,
            err_check: None,
            stdout_buf: std::sync::Arc::new(std::sync::Mutex::new(Vec::new())),
            stderr_buf: std::sync::Arc::new(std::sync::Mutex::new(Vec::new())),
            buffer_count: 0,
            buffer_rx: None,
            #[cfg(windows)]
            platform: super::platform_windows::PlatformData::new(),
        }
    }

    /// Launch all buffer copy tasks in background threads.
    pub fn setup_redirection_buffers(&mut self) {
        let tasks: Vec<BufferTask> = self.start_after_start.drain(..).collect();
        if tasks.is_empty() {
            return;
        }
        self.buffer_count = tasks.len();
        let (tx, rx) = mpsc::channel();
        for task in tasks {
            let tx = tx.clone();
            std::thread::spawn(move || {
                let result = task();
                let _ = tx.send(result);
            });
        }
        self.buffer_rx = Some(rx);
    }

    /// Wait for all buffer tasks to complete.
    pub fn collect_buffers(&mut self) {
        if let Some(rx) = self.buffer_rx.take() {
            for _ in 0..self.buffer_count {
                match rx.recv() {
                    Ok(Ok(())) => {}
                    Ok(Err(e)) => {
                        tracing::error!("buffer task error: {}", e);
                    }
                    Err(_) => break,
                }
            }
        }
    }

    /// Set up memory-based output capture. Returns the write end of a pipe.
    /// A background thread reads from the pipe into a shared buffer.
    pub fn setup_output_memory(
        &mut self,
        is_stderr: bool,
        max_output_size: i64,
    ) -> Result<File> {
        let (reader, writer) = os_pipe::pipe().context("creating pipe for output memory")?;
        let max = if max_output_size <= 0 {
            MAX_MEM_OUTPUT
        } else {
            max_output_size
        };

        let writer_file: File = unsafe { File::from_raw_handle(writer.into_raw_handle()) };
        let reader_file: File = unsafe { File::from_raw_handle(reader.into_raw_handle()) };

        // Close the writer after process creation
        self.close_after_start.push(writer_file.try_clone()?);

        let cleanup_reader = reader_file.try_clone()?;
        self.cleanup_if_failed.push(Box::new(move || {
            drop(cleanup_reader);
        }));

        let buf = if is_stderr {
            self.stderr_buf.clone()
        } else {
            self.stdout_buf.clone()
        };

        self.start_after_start.push(Box::new(move || {
            let mut limited = reader_file.take(max as u64);
            let mut data = Vec::new();
            limited.read_to_end(&mut data)?;
            *buf.lock().unwrap() = data;
            Ok(())
        }));

        Ok(writer_file)
    }

    /// Open a file for redirect. For output with size limits, sets up an OutputRedirectCheck.
    pub fn setup_file(
        &mut self,
        filename: &str,
        read: bool,
        max_output_size: i64,
        is_stderr: bool,
    ) -> Result<File> {
        let file = open_file_for_redirect(filename, read)?;
        self.close_after_start.push(file.try_clone()?);

        if max_output_size < 0 || read {
            return Ok(file);
        }

        let max = if max_output_size == 0 {
            MAX_MEM_OUTPUT
        } else {
            max_output_size
        };

        let check_file = open_file_for_check(filename)
            .with_context(|| format!("opening {:?} for size check", filename))?;
        let check = OutputRedirectCheck {
            name: filename.to_string(),
            file: check_file,
            max_size: max,
        };

        if is_stderr {
            self.err_check = Some(check);
        } else {
            self.out_check = Some(check);
        }

        Ok(file)
    }

    /// Set up a pipe redirect (pipe handle provided externally).
    pub fn setup_pipe(&mut self, file: File) -> Result<File> {
        self.close_after_start.push(file.try_clone()?);
        Ok(file)
    }

    /// Set up output redirect based on mode.
    pub fn setup_output(
        &mut self,
        redirect: Option<&Redirect>,
        is_stderr: bool,
    ) -> Result<Option<File>> {
        let r = match redirect {
            None => return Ok(None),
            Some(r) => r,
        };

        match r.mode {
            RedirectMode::None => Ok(None),
            RedirectMode::Memory => {
                let f = self.setup_output_memory(is_stderr, r.max_output_size)?;
                Ok(Some(f))
            }
            RedirectMode::File => {
                let name = r.filename.as_deref().unwrap_or("");
                let f = self.setup_file(name, false, r.max_output_size, is_stderr)?;
                Ok(Some(f))
            }
            RedirectMode::Pipe => {
                if let Some(ref pipe) = r.pipe {
                    let f = self.setup_pipe(pipe.try_clone()?)?;
                    Ok(Some(f))
                } else {
                    Ok(None)
                }
            }
            RedirectMode::Remote => {
                // Remote mode not yet implemented
                Ok(None)
            }
        }
    }

    /// Set up input redirect based on mode.
    pub fn setup_input(&mut self, redirect: Option<&Redirect>) -> Result<Option<File>> {
        let r = match redirect {
            None => return Ok(None),
            Some(r) => r,
        };

        match r.mode {
            RedirectMode::None => Ok(None),
            RedirectMode::Memory => {
                let f = self.setup_input_memory(&r.data)?;
                Ok(Some(f))
            }
            RedirectMode::File => {
                let name = r.filename.as_deref().unwrap_or("");
                let f = self.setup_file(name, true, 0, false)?;
                Ok(Some(f))
            }
            RedirectMode::Pipe => {
                if let Some(ref pipe) = r.pipe {
                    let f = self.setup_pipe(pipe.try_clone()?)?;
                    Ok(Some(f))
                } else {
                    Ok(None)
                }
            }
            RedirectMode::Remote => Ok(None),
        }
    }

    /// Set up memory-based input. Returns the read end of a pipe.
    /// A background thread writes the data into the pipe.
    pub fn setup_input_memory(&mut self, data: &[u8]) -> Result<File> {
        let (reader, writer) = os_pipe::pipe().context("creating pipe for input memory")?;
        let reader_file: File = unsafe { File::from_raw_handle(reader.into_raw_handle()) };
        let writer_file: File = unsafe { File::from_raw_handle(writer.into_raw_handle()) };

        self.close_after_start.push(reader_file.try_clone()?);

        let data_copy = data.to_vec();
        self.start_after_start.push(Box::new(move || {
            let mut w = writer_file;
            w.write_all(&data_copy)?;
            drop(w); // Close write end so reader sees EOF
            Ok(())
        }));

        Ok(reader_file)
    }
}

/// Open a file for redirect (read or write) with shared access on Windows.
#[cfg(windows)]
pub fn open_file_for_redirect(name: &str, read: bool) -> Result<File> {
    use windows_sys::Win32::Storage::FileSystem::*;
    let share = FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE;
    if read {
        OpenOptions::new()
            .read(true)
            .share_mode(share)
            .open(name)
            .with_context(|| format!("opening {:?} for reading", name))
    } else {
        OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .share_mode(share)
            .open(name)
            .with_context(|| format!("opening {:?} for writing", name))
    }
}

/// Open a file for redirect — non-Windows fallback.
#[cfg(not(windows))]
pub fn open_file_for_redirect(name: &str, read: bool) -> Result<File> {
    if read {
        File::open(name).with_context(|| format!("opening {:?} for reading", name))
    } else {
        File::create(name).with_context(|| format!("opening {:?} for writing", name))
    }
}

/// Open a file for size checking (read-only, shared on Windows).
#[cfg(windows)]
pub fn open_file_for_check(name: &str) -> Result<File> {
    use windows_sys::Win32::Storage::FileSystem::*;
    OpenOptions::new()
        .read(true)
        .share_mode(FILE_SHARE_READ | FILE_SHARE_WRITE | FILE_SHARE_DELETE)
        .open(name)
        .with_context(|| format!("opening {:?} for size check", name))
}

/// Open a file for size checking — non-Windows fallback.
#[cfg(not(windows))]
pub fn open_file_for_check(name: &str) -> Result<File> {
    File::open(name).with_context(|| format!("opening {:?} for size check", name))
}
