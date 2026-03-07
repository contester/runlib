//! Remote storage backend for file transfers (HTTP-based filer).

use std::path::Path;

use anyhow::{Result, bail};
use contester_tools::FileStat;

/// Copy a file between local filesystem and a remote HTTP filer.
///
/// If `to_remote` is true, uploads local → remote (PUT).
/// If `to_remote` is false, downloads remote → local (GET).
pub async fn filer_copy(
    client: &reqwest::Client,
    local_name: &str,
    remote_name: &str,
    to_remote: bool,
    checksum: &str,
    module_type: &str,
    auth_token: &str,
) -> Result<Option<FileStat>> {
    let remote = remote_name.strip_prefix("filer:").unwrap_or(remote_name);

    if to_remote {
        filer_upload(client, local_name, remote, checksum, module_type, auth_token).await
    } else {
        filer_download(client, local_name, remote, auth_token).await
    }
}

async fn filer_upload(
    client: &reqwest::Client,
    local_name: &str,
    remote_url: &str,
    checksum: &str,
    module_type: &str,
    auth_token: &str,
) -> Result<Option<FileStat>> {
    let local = local_name.to_string();
    let stat = tokio::task::spawn_blocking(move || {
        contester_tools::stat_file(Path::new(&local), true)
    }).await??;

    if !checksum.is_empty() && stat.checksum.as_deref() != Some(checksum) {
        bail!(
            "Checksum mismatch, local {:?} != {:?}",
            stat.checksum,
            checksum
        );
    }

    let data = tokio::fs::read(local_name).await?;
    let file_size = stat.size;

    let mut req = client.put(remote_url).body(data);

    if !module_type.is_empty() {
        req = req.header("X-FS-Module-Type", module_type);
    }
    if !auth_token.is_empty() {
        req = req.header("Authorization", format!("bearer {auth_token}"));
    }
    req = req.header("X-FS-Content-Length", file_size.to_string());

    let resp = req.send().await?;
    if !resp.status().is_success() {
        bail!("upload failed: {}", resp.status());
    }

    Ok(Some(stat))
}

async fn filer_download(
    client: &reqwest::Client,
    local_name: &str,
    remote_url: &str,
    auth_token: &str,
) -> Result<Option<FileStat>> {
    let mut req = client.get(remote_url);
    if !auth_token.is_empty() {
        req = req.header("Authorization", format!("bearer {auth_token}"));
    }

    let resp = req.send().await?;
    if resp.status() == reqwest::StatusCode::NOT_FOUND {
        bail!("not found: {remote_url:?}");
    }
    if !resp.status().is_success() {
        bail!("download failed: {}", resp.status());
    }

    let expected_len = resp.content_length();
    let bytes = resp.bytes().await?;

    if let Some(expected) = expected_len {
        if bytes.len() as u64 != expected {
            bail!("incomplete read {} want {}", bytes.len(), expected);
        }
    }

    tokio::fs::write(local_name, &bytes).await?;

    let local = local_name.to_string();
    let stat = tokio::task::spawn_blocking(move || {
        contester_tools::stat_file(Path::new(&local), true)
    }).await??;
    Ok(Some(stat))
}
