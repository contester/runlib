//! Sandbox management — directory structure and path resolution.

use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};

use anyhow::{Result, bail};

pub struct Sandbox {
    pub path: String,
}

pub struct SandboxPair {
    pub compile: Arc<Mutex<Sandbox>>,
    pub run: Arc<Mutex<Sandbox>>,
}

impl SandboxPair {
    pub fn new(base: &str) -> Self {
        let base = Path::new(base);
        Self {
            compile: Arc::new(Mutex::new(Sandbox {
                path: base.join("C").to_string_lossy().into_owned(),
            })),
            run: Arc::new(Mutex::new(Sandbox {
                path: base.join("R").to_string_lossy().into_owned(),
            })),
        }
    }
}

/// Parse a sandbox ID like "%0.C" and return the corresponding Sandbox.
pub fn get_sandbox_by_id<'a>(
    sandboxes: &'a [SandboxPair],
    id: &str,
) -> Result<&'a Arc<Mutex<Sandbox>>> {
    if id.len() < 4 || !id.starts_with('%') {
        bail!("Malformed sandbox ID {id}");
    }

    let rest = &id[1..];
    let parts: Vec<&str> = rest.split('.').collect();
    if parts.len() != 2 {
        bail!("Malformed sandbox ID {id}");
    }

    let index: usize = parts[0]
        .parse()
        .map_err(|_| anyhow::anyhow!("Can't parse sandbox index {:?}", parts[0]))?;

    if index >= sandboxes.len() {
        bail!(
            "Sandbox index {} out of range (max={})",
            index,
            sandboxes.len()
        );
    }

    match parts[1].to_uppercase().as_str() {
        "C" => Ok(&sandboxes[index].compile),
        "R" => Ok(&sandboxes[index].run),
        other => bail!("Sandbox variant {other} is unknown"),
    }
}

/// Find a sandbox whose path is a prefix of the given path.
pub fn get_sandbox_by_path<'a>(
    sandboxes: &'a [SandboxPair],
    path: &str,
) -> Result<&'a Arc<Mutex<Sandbox>>> {
    let clean = PathBuf::from(path);
    let clean = clean.to_string_lossy();

    for pair in sandboxes {
        if clean.starts_with(&pair.compile.lock().unwrap().path) {
            return Ok(&pair.compile);
        }
        if clean.starts_with(&pair.run.lock().unwrap().path) {
            return Ok(&pair.run);
        }
    }
    bail!("No sandbox corresponds to path {path}");
}

/// Resolve a path that may start with a sandbox ID (%N.X) or be absolute.
/// Returns (resolved_path, optional sandbox reference).
pub fn resolve_path<'a>(
    sandboxes: &'a [SandboxPair],
    source: &str,
    restricted: bool,
) -> Result<(String, Option<&'a Arc<Mutex<Sandbox>>>)> {
    if source.is_empty() {
        bail!("Invalid path (empty)");
    }

    if source.starts_with('%') {
        let sep = std::path::MAIN_SEPARATOR;
        let parts: Vec<&str> = source.splitn(2, sep).collect();
        let sandbox = get_sandbox_by_id(sandboxes, parts[0])?;
        let sandbox_path = sandbox.lock().unwrap().path.clone();

        if parts.len() == 2 {
            let resolved = Path::new(&sandbox_path).join(parts[1]);
            return Ok((resolved.to_string_lossy().into_owned(), Some(sandbox)));
        }
        return Ok((sandbox_path, Some(sandbox)));
    }

    if !Path::new(source).is_absolute() {
        bail!("Relative path {source}");
    }

    if restricted {
        let sandbox = get_sandbox_by_path(sandboxes, source)?;
        return Ok((source.to_string(), Some(sandbox)));
    }

    Ok((source.to_string(), None))
}
