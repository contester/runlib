# Rust Rewrite Plan for contester-runlib

## Cargo Workspace Layout

```
rust/
├── Cargo.toml                    # workspace root
├── proto/                        # .proto files copied from Go
│   ├── Blobs.proto
│   ├── Execution.proto
│   ├── Local.proto
│   └── Contester.proto
├── crates/
│   ├── contester-proto/          # generated protobuf types + blob helpers
│   │   ├── Cargo.toml
│   │   ├── build.rs              # prost-build
│   │   └── src/lib.rs
│   ├── tools/                    # utilities (sha1, file stat)
│   │   ├── Cargo.toml
│   │   └── src/lib.rs
│   ├── subprocess/               # core execution engine
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs            # Subprocess, SubprocessResult, flags, Redirect
│   │       ├── platform_windows.rs  # CreateFrozen, BottomHalf, Job Objects
│   │       ├── platform_linux.rs    # stub for now (cgroups+clone later)
│   │       ├── redirects.rs      # I/O redirect setup
│   │       └── interconnect.rs   # interactive pipe plumbing + interaction log
│   ├── rpc4/                     # wire protocol codec
│   │   ├── Cargo.toml
│   │   └── src/lib.rs
│   ├── service/                  # RPC method implementations
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs            # Contester struct, config, Identify
│   │       ├── exec.rs           # LocalExecute, LocalExecuteConnected
│   │       ├── sandbox.rs        # sandbox management, user creation
│   │       ├── sandbox_windows.rs
│   │       ├── sandbox_linux.rs
│   │       ├── fileops.rs        # Stat, Get, Put, Clear
│   │       └── gridfs.rs         # GridfsCopy (HTTP transfers)
│   └── storage/                  # remote storage backend
│       ├── Cargo.toml
│       └── src/lib.rs
└── bins/
    ├── runexe/                   # CLI binary
    │   ├── Cargo.toml
    │   └── src/
    │       ├── main.rs
    │       ├── flags.rs          # clap definitions
    │       └── results.rs        # verdict, text/XML output
    └── runner/                   # daemon binary
        ├── Cargo.toml
        └── src/main.rs
```

## Dependency Graph

```
contester-proto  ←── tools
       ↑                ↑
   subprocess ──────────┘
       ↑
   service ────→ storage
       ↑
   rpc4
       ↑
   runner (bin)

subprocess
       ↑
   runexe (bin)
```

## Key Crate Choices

| Purpose | Crate | Rationale |
|---------|-------|-----------|
| Protobuf codegen | `prost` + `prost-build` | Idiomatic Rust, widely used, generates simple structs |
| Windows API | `windows-sys` | Low-level FFI bindings, lighter than `windows-rs`, appropriate for direct syscall work |
| CLI parsing | `clap` (derive) | Standard, supports the flag style runexe uses |
| Logging | `tracing` + `tracing-subscriber` | Structured logging, modern standard |
| Config | `toml` + `serde` | Direct match for the TOML config format |
| Hashing | `sha1` crate | Simple SHA-1 for blob checksums |
| Compression | `flate2` | zlib compression for Blob |
| Async runtime | `tokio` (full features) | Future-proofing, async RPC + HTTP |
| HTTP client | `reqwest` | Async HTTP client for GridFS transfers |
| XML output | `quick-xml` + `serde` | For runexe XML result format |
| Byte order | `byteorder` | For rpc4 length-prefix framing |

## Async Decision

**Async with tokio** from the start for future-proofing:

- Runner daemon: `tokio::net::TcpStream` for the dispatcher connection, async read/write for RPC framing
- RPC4 codec: async reader/writer using `tokio::io::{AsyncReadExt, AsyncWriteExt}`
- Service methods: `async fn` signatures, allowing future concurrency
- HTTP transfers (GridFS): use `reqwest` (async) instead of `ureq`
- Subprocess monitoring loop: runs on a **blocking thread** via `tokio::task::spawn_blocking` — the tight 250ms poll with Win32 `WaitForSingleObject` is inherently blocking and doesn't benefit from async, but wrapping it lets the rest of the system stay async
- Interactive mode pipe I/O: `tokio::task::spawn_blocking` for the pipe copy threads
- runexe CLI: uses `#[tokio::main]` even though it's mostly sequential — keeps the subprocess API consistent and avoids needing both sync and async variants

## Implementation Phases

### Phase 1: Scaffolding + Protobuf (est. ~1 session)

**What**: Set up the workspace, generate Rust types from .proto files, implement Blob helpers.

**Steps**:
1. Create workspace `Cargo.toml` and all crate skeletons
2. Copy `.proto` files to `rust/proto/`
3. Set up `contester-proto` crate with `prost-build` in `build.rs`
4. Implement `Blob` helper methods: `new_blob(data) → Blob` (zlib compress + SHA-1), `blob_bytes(blob) → Vec<u8>` (decompress), `blob_reader(blob) → impl Read`
5. Implement `tools` crate: `hash_file()`, `stat_file()`

**Test**: Unit tests for Blob round-trip (compress → decompress, SHA-1 verification).

**Deliverable**: `cargo build` succeeds, proto types compile, blob helpers work.

### Phase 2: Subprocess Engine — Windows (est. ~3-4 sessions)

**What**: Port the core execution engine. This is the hardest phase.

**Steps**:
1. Define core types in `subprocess` crate:
   - `Subprocess` struct (config, immutable)
   - `SubprocessData` struct (mutable runtime state)
   - `SubprocessResult` struct
   - `Redirect` enum
   - `CommandLine` struct
   - Execution flags constants (`EF_*`)
   - `RedirectMode` enum

2. Implement redirect infrastructure (`redirects.rs`):
   - `setup_input()` / `setup_output()` — file, memory, pipe modes
   - `setup_output_memory()` — pipe + thread to collect output
   - Output size checking

3. Implement Windows process creation (`platform_windows.rs`):
   - `LoginInfo` — `LogonUserW`, `LoadUserProfileW` via `windows-sys`
   - `create_frozen()`:
     - Build `STARTUPINFOW` with redirect handles
     - `CreateProcessWithLogonW` or `CreateProcessW` with `CREATE_SUSPENDED`
     - Job Object creation + configuration (UI restrictions, hard limits)
     - Process affinity
   - `unfreeze()` — `ResumeThread`
   - `bottom_half()` — monitoring loop:
     - `WaitForSingleObject` with timeout
     - `GetProcessTimes` / Job Object accounting
     - Memory measurement via `QueryInformationJobObject`
     - Soft limit checking + idleness detection
     - `TerminateProcess` when limits exceeded
     - Post-exit limit check

4. Implement `Execute()` orchestrating the lifecycle

5. Create Linux stub (`platform_linux.rs`):
   - `create_frozen()` → `unimplemented!()` or compile error
   - Just enough to make `cargo check` pass on Linux targets

**Test**: Build `cpuhog` equivalent in Rust, run it with time+memory limits via a simple test binary. Verify time limit detection, memory measurement, exit code collection.

### Phase 3: runexe CLI (est. ~1-2 sessions)

**What**: Port the CLI tool so we can test subprocess execution end-to-end.

**Steps**:
1. Define clap CLI matching Go's flag scheme (`-t`, `-m`, `-d`, `-i`, `-o`, etc.)
2. Implement flag types: `TimeLimitFlag` (parse "1s", "500ms", "2000"), `MemoryLimitFlag` (parse "256M", "1G", raw bytes)
3. Port `SetupSubprocess` — translate CLI flags to `Subprocess` config
4. Port verdict determination logic (priority-ordered match on `SuccessCode`)
5. Port text output format (matching Go output exactly)
6. Port XML output format (matching Go XML schema exactly)
7. Implement `--interactor` support:
   - Port `commandLineToArgv` (Windows command-line splitting)
   - Port `Interconnect` for pipe plumbing
   - Port interaction log writer (`InteractionLog` with `< ` / `> ` / `\n~` markers)
   - Concurrent execution of both processes

**Test**: Run the Rust `runexe` alongside the Go `runexe` on the same test programs, compare output. Test interactive mode with `intersink`.

### Phase 4: RPC4 Wire Protocol (est. ~1 session)

**What**: Implement the custom length-prefixed protobuf codec.

**Steps**:
1. Define `Header` protobuf (sequence, message_type, payload_present, method) — can be a separate small .proto or hand-coded prost struct
2. Implement frame reading: read 4-byte big-endian length, read that many bytes
3. Implement frame writing: write 4-byte big-endian length, write data
4. Implement `read_request()`: read header frame, optionally read payload frame
5. Implement `write_response()`: write header frame (RESPONSE or ERROR), write payload or error string
6. Handle the error case: ERROR message type + raw UTF-8 string payload (not protobuf)

**Test**: Unit tests with captured wire data from the Go implementation. Or: integration test connecting to a test Scala dispatcher.

### Phase 5: Service + Runner (est. ~2-3 sessions)

**What**: Port the sandbox management, RPC methods, and runner daemon.

**Steps**:
1. Port sandbox management:
   - `Sandbox`, `SandboxPair` structs with `Mutex`
   - Sandbox ID parsing (`%0.C` → path resolution)
   - Directory creation
   - Windows: user creation (`NetUserAdd`), ACL setup, password management
   - `ClearSandbox` with retry loop

2. Port `Contester` service struct and config loading (TOML)

3. Port RPC methods:
   - `Identify` — return capabilities
   - `LocalExecute` — single execution
   - `LocalExecuteConnected` — interactive execution
   - `Put` / `Get` — file transfer
   - `Stat` — file metadata (with glob expansion)
   - `GridfsCopy` — HTTP file transfers
   - `Clear` — sandbox wipe

4. Port runner daemon:
   - TCP connect to dispatcher
   - Serve RPC requests using rpc4 codec
   - Reconnect loop with 5s backoff
   - CLI flags (logfile, config file)

**Test**: Connect to actual Scala dispatcher, run `Identify`, execute a simple program remotely.

## Platform Abstraction Strategy

Use `cfg` attributes at the module level:

```rust
// subprocess/src/lib.rs
#[cfg(windows)]
mod platform_windows;
#[cfg(unix)]
mod platform_linux;

#[cfg(windows)]
use platform_windows as platform;
#[cfg(unix)]
use platform_linux as platform;
```

The platform modules export the same function signatures:
- `create_frozen(subprocess: &Subprocess) -> Result<SubprocessData>`
- `unfreeze(data: &mut SubprocessData)`
- `bottom_half(subprocess: &Subprocess, data: &mut SubprocessData) -> SubprocessResult`

No trait objects needed — compile-time dispatch via `cfg` is simpler and zero-cost.

Same pattern for `service/sandbox_windows.rs` and `service/sandbox_linux.rs`.

## Windows API Approach

Use `windows-sys` with feature flags for the specific API sets needed:

```toml
[target.'cfg(windows)'.dependencies]
windows-sys = { version = "0.59", features = [
    "Win32_System_Threading",        # CreateProcess, Job Objects, ResumeThread
    "Win32_System_JobObjects",       # Job Object configuration
    "Win32_Security",                # LogonUser, ACLs
    "Win32_System_Memory",           # VirtualAllocEx, WriteProcessMemory
    "Win32_System_ProcessStatus",    # GetProcessMemoryInfo
    "Win32_UI_WindowsAndMessaging",  # CreateDesktop, WindowStation
    "Win32_System_UserProfile",      # LoadUserProfile
    "Win32_NetworkManagement_NetManagement",  # NetUserAdd
]}
```

All Win32 calls wrapped in safe Rust functions in the platform module, with proper RAII for handles (implement `Drop` for `HANDLE` wrappers).

## DLL Injection

Port directly using `windows-sys`:
1. `GetBinaryTypeW` to detect 32/64-bit target
2. `VirtualAllocEx` + `WriteProcessMemory` to write DLL path
3. `CreateRemoteThread` with `LoadLibraryW` as entry point

This is Phase 2 work but can be deferred to a sub-phase since it's only needed for specific use cases.

## What We Build First

**Phase 1** — because everything depends on the proto types. This is quick and verifies the build setup works.
