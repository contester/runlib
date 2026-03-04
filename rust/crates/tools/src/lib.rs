use std::path::Path;

use anyhow::Result;
use sha1::{Digest, Sha1};

/// Compute the SHA-1 hash of a file, returning raw bytes.
pub fn hash_file(path: &Path) -> Result<[u8; 20]> {
    let data = std::fs::read(path)?;
    let mut hasher = Sha1::new();
    hasher.update(&data);
    Ok(hasher.finalize().into())
}

/// Compute the SHA-1 hash of a file, returning "sha1:<hex>" string.
pub fn hash_file_string(path: &Path) -> Result<String> {
    let hash = hash_file(path)?;
    Ok(format!("sha1:{}", hex_encode(&hash)))
}

/// Encode bytes as lowercase hex string.
pub fn hex_encode(bytes: &[u8]) -> String {
    bytes.iter().map(|b| format!("{b:02x}")).collect()
}

/// File metadata with optional checksum.
pub struct FileStat {
    pub name: String,
    pub is_directory: bool,
    pub size: u64,
    pub checksum: Option<String>,
}

/// Get file metadata, optionally computing checksum.
pub fn stat_file(path: &Path, calculate_checksum: bool) -> Result<FileStat> {
    let meta = std::fs::metadata(path)?;
    let checksum = if calculate_checksum && !meta.is_dir() {
        Some(hash_file_string(path)?)
    } else {
        None
    };
    Ok(FileStat {
        name: path.to_string_lossy().into_owned(),
        is_directory: meta.is_dir(),
        size: meta.len(),
        checksum,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_hex_encode() {
        assert_eq!(hex_encode(&[0xde, 0xad, 0xbe, 0xef]), "deadbeef");
    }
}
