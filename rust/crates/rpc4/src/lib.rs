//! RPC4 wire protocol codec.
//!
//! A simple length-prefixed protobuf protocol used for reverse-RPC between
//! the runner and the dispatcher.
//!
//! Wire format:
//! - Each frame is: 4-byte big-endian length + that many bytes of data
//! - A request consists of: header frame + optional payload frame
//! - A response consists of: header frame + optional payload frame
//! - For ERROR responses, the payload is raw UTF-8 text (not protobuf)

use anyhow::{Context, Result, bail};
use prost::Message;
use tokio::io::{AsyncRead, AsyncReadExt, AsyncWrite, AsyncWriteExt, BufReader, BufWriter};

// ── Header message ──────────────────────────────────────────────────────────

/// RPC4 header message type.
#[derive(Clone, Copy, Debug, PartialEq, Eq, prost::Enumeration)]
#[repr(i32)]
pub enum MessageType {
    Request = 1,
    Response = 2,
    Error = 3,
    Cancel = 4,
}

/// RPC4 header — hand-coded prost struct matching rpcfour.proto.
#[derive(Clone, prost::Message)]
pub struct Header {
    #[prost(uint64, optional, tag = "1")]
    pub sequence: Option<u64>,
    #[prost(enumeration = "MessageType", optional, tag = "2")]
    pub message_type: Option<i32>,
    #[prost(bool, optional, tag = "3")]
    pub payload_present: Option<bool>,
    #[prost(string, optional, tag = "4")]
    pub method: Option<String>,
}

impl Header {
    pub fn get_message_type(&self) -> MessageType {
        match self.message_type {
            Some(1) => MessageType::Request,
            Some(2) => MessageType::Response,
            Some(3) => MessageType::Error,
            Some(4) => MessageType::Cancel,
            _ => MessageType::Request,
        }
    }
}

// ── Incoming request (parsed from wire) ─────────────────────────────────────

/// A parsed RPC request from the dispatcher.
pub struct RpcRequest {
    pub method: String,
    pub sequence: u64,
    /// Raw protobuf-encoded payload bytes (if present).
    pub payload: Option<Vec<u8>>,
}

/// A parsed RPC response (for client-side usage).
pub struct RpcResponse {
    pub method: String,
    pub sequence: u64,
    pub is_error: bool,
    /// For ERROR: raw UTF-8 string. For RESPONSE: protobuf bytes.
    pub payload: Option<Vec<u8>>,
}

// ── Frame I/O ───────────────────────────────────────────────────────────────

/// Read a length-prefixed frame from a reader.
async fn read_frame<R: AsyncRead + Unpin>(r: &mut R) -> Result<Vec<u8>> {
    let size = r.read_u32().await.context("reading frame length")?;
    let mut buf = vec![0u8; size as usize];
    r.read_exact(&mut buf).await.context("reading frame data")?;
    Ok(buf)
}

/// Write a length-prefixed frame to a writer.
async fn write_frame<W: AsyncWrite + Unpin>(w: &mut W, data: &[u8]) -> Result<()> {
    w.write_u32(data.len() as u32).await.context("writing frame length")?;
    w.write_all(data).await.context("writing frame data")?;
    Ok(())
}

/// Read and decode a protobuf message from a length-prefixed frame.
async fn read_proto<R: AsyncRead + Unpin, M: Message + Default>(r: &mut R) -> Result<M> {
    let buf = read_frame(r).await?;
    M::decode(buf.as_slice()).context("decoding protobuf message")
}

/// Encode and write a protobuf message as a length-prefixed frame.
async fn write_proto<W: AsyncWrite + Unpin, M: Message>(w: &mut W, msg: &M) -> Result<()> {
    let data = msg.encode_to_vec();
    write_frame(w, &data).await
}

// ── Server codec ────────────────────────────────────────────────────────────

/// RPC4 server codec — reads requests and writes responses on a TCP connection.
pub struct ServerCodec<R, W> {
    reader: BufReader<R>,
    writer: BufWriter<W>,
}

impl<R: AsyncRead + Unpin, W: AsyncWrite + Unpin> ServerCodec<R, W> {
    pub fn new(reader: R, writer: W) -> Self {
        Self {
            reader: BufReader::new(reader),
            writer: BufWriter::new(writer),
        }
    }

    /// Read the next RPC request from the wire.
    pub async fn read_request(&mut self) -> Result<RpcRequest> {
        let header: Header = read_proto(&mut self.reader).await?;

        let method = header.method.unwrap_or_default();
        if method.is_empty() {
            bail!("header missing method");
        }

        let sequence = header.sequence.unwrap_or(0);
        let has_payload = header.payload_present.unwrap_or(false);

        let payload = if has_payload {
            Some(read_frame(&mut self.reader).await?)
        } else {
            None
        };

        Ok(RpcRequest {
            method,
            sequence,
            payload,
        })
    }

    /// Write a successful response with a protobuf payload.
    pub async fn write_response<M: Message>(
        &mut self,
        method: &str,
        sequence: u64,
        payload: &M,
    ) -> Result<()> {
        let data = payload.encode_to_vec();

        let header = Header {
            method: Some(method.to_string()),
            sequence: Some(sequence),
            message_type: Some(MessageType::Response as i32),
            payload_present: if data.is_empty() { None } else { Some(true) },
        };

        write_proto(&mut self.writer, &header).await?;
        if !data.is_empty() {
            write_frame(&mut self.writer, &data).await?;
        }
        self.writer.flush().await.context("flushing response")?;
        Ok(())
    }

    /// Write an error response with a UTF-8 error message.
    pub async fn write_error(
        &mut self,
        method: &str,
        sequence: u64,
        error: &str,
    ) -> Result<()> {
        let data = error.as_bytes();

        let header = Header {
            method: Some(method.to_string()),
            sequence: Some(sequence),
            message_type: Some(MessageType::Error as i32),
            payload_present: if data.is_empty() { None } else { Some(true) },
        };

        write_proto(&mut self.writer, &header).await?;
        if !data.is_empty() {
            write_frame(&mut self.writer, data).await?;
        }
        self.writer.flush().await.context("flushing error response")?;
        Ok(())
    }

    /// Flush the underlying writer.
    pub async fn flush(&mut self) -> Result<()> {
        self.writer.flush().await.context("flushing writer")?;
        Ok(())
    }
}

/// Create a server codec from a tokio TcpStream (splits into read/write halves).
impl ServerCodec<tokio::net::tcp::OwnedReadHalf, tokio::net::tcp::OwnedWriteHalf> {
    pub fn from_tcp(stream: tokio::net::TcpStream) -> Self {
        let (reader, writer) = stream.into_split();
        Self::new(reader, writer)
    }
}
