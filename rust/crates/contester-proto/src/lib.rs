use std::io::Read;

use anyhow::{Context, Result};
use flate2::read::ZlibDecoder;
use flate2::write::ZlibEncoder;
use flate2::Compression;
use sha1::{Digest, Sha1};
use std::io::Write;

/// Generated protobuf types.
pub mod proto {
    include!(concat!(env!("OUT_DIR"), "/contester.proto.rs"));
}

// Re-export commonly used types at crate root.
pub use proto::*;

impl Blob {
    /// Create a new Blob from raw data, compressing with zlib and computing SHA-1.
    pub fn new(data: &[u8]) -> Result<Option<Self>> {
        if data.is_empty() {
            return Ok(None);
        }

        let sha1_hash = {
            let mut hasher = Sha1::new();
            hasher.update(data);
            hasher.finalize().to_vec()
        };

        let compressed = compress(data)?;

        Ok(Some(Blob {
            data: compressed,
            compression: Some(blob::CompressionInfo {
                method: blob::compression_info::CompressionType::MethodZlib as i32,
                original_size: data.len() as u32,
            }),
            sha1: sha1_hash,
        }))
    }

    /// Get a reader that decompresses the blob data.
    pub fn reader(&self) -> Result<Box<dyn Read + '_>> {
        match self.compression.as_ref() {
            Some(info)
                if info.method
                    == blob::compression_info::CompressionType::MethodZlib as i32 =>
            {
                Ok(Box::new(ZlibDecoder::new(self.data.as_slice())))
            }
            _ => Ok(Box::new(self.data.as_slice())),
        }
    }

    /// Decompress and return all bytes.
    pub fn bytes(&self) -> Result<Vec<u8>> {
        let mut reader = self.reader()?;
        let mut buf = Vec::new();
        reader
            .read_to_end(&mut buf)
            .context("decompressing blob")?;
        Ok(buf)
    }
}

/// Compress data using zlib.
fn compress(data: &[u8]) -> Result<Vec<u8>> {
    let mut encoder = ZlibEncoder::new(Vec::new(), Compression::default());
    encoder.write_all(data)?;
    Ok(encoder.finish()?)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_blob_roundtrip() {
        let original = b"Hello, contester! This is test data for blob compression.";
        let blob = Blob::new(original).unwrap().unwrap();

        // Verify SHA-1
        let mut hasher = Sha1::new();
        hasher.update(original);
        let expected_sha1 = hasher.finalize().to_vec();
        assert_eq!(blob.sha1, expected_sha1);

        // Verify compression info
        let info = blob.compression.as_ref().unwrap();
        assert_eq!(
            info.method,
            blob::compression_info::CompressionType::MethodZlib as i32
        );
        assert_eq!(info.original_size, original.len() as u32);

        // Verify roundtrip
        let decoded = blob.bytes().unwrap();
        assert_eq!(decoded, original);
    }

    #[test]
    fn test_blob_empty() {
        let blob = Blob::new(b"").unwrap();
        assert!(blob.is_none());
    }

    #[test]
    fn test_blob_reader() {
        let original = b"test data";
        let blob = Blob::new(original).unwrap().unwrap();
        let mut reader = blob.reader().unwrap();
        let mut buf = String::new();
        reader.read_to_string(&mut buf).unwrap();
        assert_eq!(buf.as_bytes(), original);
    }
}
