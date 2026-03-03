# runlib Specification

This document specifies the architecture, protocols, and behavior of the `runlib` system — an execution backend for programming competitions. The system allows sandboxed execution of untrusted programs with precise resource measurement and enforcement, with support for both local (CLI) and remote (RPC service) operation.

## 1. System Overview

### 1.1 Purpose

`runlib` provides:
1. **Sandboxed process execution** with time, memory, and process-count limits
2. **Precise resource measurement** (user CPU time, kernel CPU time, wall time, peak memory)
3. **I/O redirection** (file, memory buffer, pipe, remote stream)
4. **Interactive problem support** — two processes connected via pipes with optional interaction logging
5. **Remote execution service** — a runner daemon that accepts execution commands from a central dispatcher via reverse RPC
6. **File transfer** — uploading/downloading files between the runner and a remote storage backend

### 1.2 Components

| Component | Type | Description |
|-----------|------|-------------|
| `runexe` | Binary | CLI tool for local sandboxed execution |
| `runner` | Binary | Daemon that connects to a dispatcher and serves RPC requests |
| `subprocess` | Library | Core process creation, monitoring, and resource enforcement |
| `service` | Library | RPC method implementations (execution, file ops, sandbox management) |
| `contester_proto` | Library | Protobuf message definitions and helpers |
| `platform` | Library | Platform abstraction (Windows desktop/global state, Linux stubs) |
| `win32` | Library | Windows API syscall wrappers |
| `linux` | Library | Linux cgroups and clone() support |
| `storage` | Library | Remote storage backend abstraction (Weed Filer) |
| `tools` | Library | Utilities (SHA-1 hashing, file stat, aligned buffers) |
| `cpuhog` | Binary | Test utility — infinite CPU loop |
| `intersink` | Binary | Test utility — stdin drain (interactor stub) |
| `problemimporter` | Binary | Imports competition problems into storage |

### 1.3 Platform Support

- **Windows** (primary): Full support including Job Objects, desktop isolation, user impersonation, DLL injection, ACL management. Supports both x86 and amd64.
- **Linux** (partial/experimental): Uses cgroups v1 and clone() with ptrace. Development was started but dropped.

---

## 2. Subprocess Execution Engine

The `subprocess` package is the core of the system. It manages process lifecycle, resource limits, I/O redirection, and result collection.

### 2.1 Configuration (`Subprocess` struct)

The `Subprocess` struct is the immutable configuration for an execution. It is not modified during execution.

```
Subprocess:
  Cmd: CommandLine
    ApplicationName: string     # Path to executable (optional on Windows if CommandLine set)
    CommandLine: string         # Full command line string
    Parameters: []string        # Alternative: structured parameter list

  CurrentDirectory: string      # Working directory for the process
  Environment: []string         # Environment variables ("KEY=VALUE" format)
  NoInheritEnvironment: bool    # If true, don't inherit parent environment

  Login: *LoginInfo             # User credentials for impersonation (platform-specific)
  Options: *PlatformOptions     # Platform-specific options (desktop, DLL injection, cgroups)

  # Resource limits (0 = unlimited)
  TimeLimit: Duration           # User-mode CPU time limit (soft)
  KernelTimeLimit: Duration     # Kernel-mode CPU time limit (soft)
  WallTimeLimit: Duration       # Wall clock time limit (soft)
  MemoryLimit: uint64           # Peak memory limit in bytes (soft, checked by monitor)
  HardMemoryLimit: uint64       # Hard memory limit (enforced by OS via job object)
  ProcessLimit: uint32          # Maximum number of active child processes

  # Behavior flags
  CheckIdleness: bool           # Enable idle process detection
  NoJob: bool                   # Windows: skip job object creation
  RestrictUi: bool              # Windows: restrict UI operations via job object
  FailOnJobCreationFailure: bool # Fatal error if job object cannot be created
  ProcessAffinityMask: uint64   # CPU affinity bitmask (0 = default)

  # I/O redirection
  StdIn: *Redirect              # stdin configuration
  StdOut: *Redirect             # stdout configuration
  StdErr: *Redirect             # stderr configuration
  JoinStdOutErr: bool           # If true, stderr goes to same destination as stdout

  # Monitoring
  TimeQuantum: Duration         # How often to check limits (default: 250ms)
```

### 2.2 Redirect Configuration

```
Redirect:
  Mode: RedirectMode
    REDIRECT_NONE   = 0   # No redirection (inherit parent)
    REDIRECT_MEMORY = 1   # Capture to/from in-memory buffer
    REDIRECT_FILE   = 2   # Read from / write to file
    REDIRECT_PIPE   = 3   # Use provided pipe file descriptor
    REDIRECT_REMOTE = 4   # Stream from remote URL

  Filename: string          # File path (for REDIRECT_FILE)
  Pipe: *os.File            # Pipe handle (for REDIRECT_PIPE)
  Data: []byte              # Input data (for REDIRECT_MEMORY input)
  MaxOutputSize: int64      # Maximum output bytes (0 = 1 GiB default)
```

**Output size enforcement**: When `MaxOutputSize > 0` for file-based output, a second file handle is opened for periodic size checking during the monitoring loop. If the file exceeds the limit, the process receives `EF_STDOUT_OVERFLOW` or `EF_STDERR_OVERFLOW`. For memory-based output, `io.LimitReader` enforces the cap.

### 2.3 Execution Lifecycle

```
Execute():
  1. maybeLockOSThread()           # Linux only: lock goroutine to OS thread for ptrace
  2. d = CreateFrozen()            # Create process in suspended state
     - On failure: run d.cleanupIfFailed functions, return error
  3. d.SetupRedirectionBuffers()   # Launch goroutines to copy I/O through pipes
  4. d.Unfreeze()                  # Resume the suspended process
  5. result = BottomHalf(d)        # Monitor loop until process exits or is killed
  6. maybeUnlockOSThread()         # Linux only: release OS thread lock
  7. return result
```

#### 2.3.1 CreateFrozen — Windows

1. Build `STARTUPINFO` with redirect handles (via `wAllRedirects`)
2. Set `STARTF_USESTDHANDLES` flag if any redirects are configured
3. Mark redirect handles as inheritable (`SetHandleInformation`)
4. Create process with `CREATE_SUSPENDED | CREATE_BREAKAWAY_FROM_JOB | CREATE_UNICODE_ENVIRONMENT`:
   - If `Login` is set: `CreateProcessWithLogonW` (with `LOGON_WITH_PROFILE`)
   - Otherwise: `CreateProcess`
5. Close parent-side redirect handles
6. If `InjectDLL` is set: inject DLLs via `VirtualAllocEx` + `WriteProcessMemory` + `CreateRemoteThread`
7. If `ProcessAffinityMask` is set: call `SetProcessAffinityMask`
8. Unless `NoJob` is true: create and configure a Job Object, assign process to it

#### 2.3.2 CreateFrozen — Linux

1. Build `CloneParams` via `linux.CreateCloneParams()` (C FFI for clone args, environment, stdio handles)
2. Call `CloneFrozen()` to create process via clone() in stopped state
3. Set up cgroup for resource tracking

#### 2.3.3 Job Object Configuration (Windows)

The Job Object enforces hard limits and provides resource accounting:

**UI Restrictions** (when `RestrictUi` is true):
- No desktop switching, display settings changes, shutdown, global atoms, handle operations, clipboard read/write, system parameter changes

**Extended Limits**:
- `JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION` — always set
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — always set
- **Hard time limit**: `max(TimeLimit + 1s, WallTimeLimit)` applied as both per-process and per-job user time limit
- **Process limit**: `ProcessLimit` → `ActiveProcessLimit` with `JOB_OBJECT_LIMIT_ACTIVE_PROCESS`
- **Hard memory limit**: `HardMemoryLimit` → `ProcessMemoryLimit`, `JobMemoryLimit`, and `MaximumWorkingSetSize`

The hard time limit is set 1 second above the soft limit to give the monitoring loop a chance to detect and report the violation before the OS terminates the process.

### 2.4 Monitoring Loop (BottomHalf)

#### Windows

```
result = SubprocessResult{}
running = runningState{}

loop while result.SuccessCode == 0:
  WaitForSingleObject(process_handle, TimeQuantum)
  if timeout (WAIT_TIMEOUT):
    UpdateProcessTimes(result)      # GetProcessTimes or JobObject accounting
    UpdateProcessMemory(result)     # JobObject PeakJobMemoryUsed or GetProcessMemoryInfo
    running.Update(sub, result)     # Check all soft limits, idleness
    Check stdout/stderr file sizes via outCheck/errCheck
  if process exited (WAIT_OBJECT_0):
    break

GetExitCodeProcess → result.ExitCode
Final UpdateProcessTimes and UpdateProcessMemory
SetPostLimits(result)               # Re-check limits after final measurement
Collect buffered output from redirection goroutines
Close file size checkers
return result
```

#### Linux

```
result = SubprocessResult{}
Start ChildWaitingFunc() goroutine  # Calls Wait4() with WUNTRACED|WCONTINUED
Create ticker at TimeQuantum interval

loop while result.SuccessCode == 0:
  select:
    case child exited:
      Record exit status, signals, rusage
      break loop
    case ticker fires:
      UpdateRunningUsage()          # Read times/memory from cgroup
      running.Update(sub, result)   # Check all soft limits, idleness

If process didn't exit naturally:
  Set EF_KILLED
  Send SIGKILL
  Wait for child

Update final stats from cgroup
Remove cgroup
SetPostLimits(result)
Collect buffered output
return result
```

### 2.5 Resource Measurement

#### Time Measurement — Windows

- **User/Kernel time**: `GetProcessTimes()` or `JobObjectBasicAccountingInformation` (preferred when job object exists — more accurate for multi-process scenarios)
- **Wall time**: Difference between process creation time and current time (or exit time)
- Windows FILETIME uses 100-nanosecond units; conversion: `duration = ns100 * 100ns`

#### Time Measurement — Linux

- **User/Kernel time**: Read from cgroup `cpuacct` stats via `GetCpu()`
- **Wall time**: `time.Since(creationTime)`

#### Memory Measurement — Windows

- With job object: `PeakJobMemoryUsed` from `JobObjectExtendedLimitInformation`
- Without job object: `max(PeakPagefileUsage, PrivateUsage)` from `GetProcessMemoryInfo`

#### Memory Measurement — Linux

- Read from cgroup `memory` stats via `GetMemory()`

### 2.6 Soft Limit Checking

Checked every `TimeQuantum` (250ms default) during monitoring:

| Condition | Flag Set |
|-----------|----------|
| `UserTime > TimeLimit` | `EF_TIME_LIMIT_HIT` |
| `KernelTime > KernelTimeLimit` | `EF_KERNEL_TIME_LIMIT_HIT` |
| `WallTime > WallTimeLimit` | `EF_WALL_TIME_LIMIT_HIT` |
| `PeakMemory > MemoryLimit` | `EF_MEMORY_LIMIT_HIT` |
| Idle for ≥6 quanta AND `WallTime > TimeLimit` | `EF_INACTIVE` |

When any flag is set (`SuccessCode != 0`), the monitoring loop exits and the process is terminated.

**Post-execution limit check** (`SetPostLimits`): After the process exits and final measurements are taken, limits are checked again and `_POST` variants of flags are set. This catches cases where the process exceeded limits between the last quantum check and termination.

### 2.7 Idleness Detection

Tracked via `runningState`:
- Each quantum, total CPU time (`UserTime + KernelTime`) is compared to previous quantum
- If unchanged, `noTimeUsedCount` increments; otherwise resets to 0
- If `noTimeUsedCount >= 6` (i.e., ≥1.5 seconds of no CPU progress) AND `WallTime > TimeLimit`: set `EF_INACTIVE`

### 2.8 Execution Result Flags (SuccessCode)

Bitmask stored in `SubprocessResult.SuccessCode`:

| Flag | Bit | Meaning |
|------|-----|---------|
| `EF_INACTIVE` | 0 | Process idle while exceeding time limit |
| `EF_TIME_LIMIT_HIT` | 1 | User time exceeded soft limit |
| `EF_WALL_TIME_LIMIT_HIT` | 2 | Wall time exceeded soft limit |
| `EF_MEMORY_LIMIT_HIT` | 3 | Peak memory exceeded soft limit |
| `EF_KILLED` | 4 | Process was killed (by monitor or OS) |
| `EF_STDOUT_OVERFLOW` | 5 | Stdout file exceeded max size |
| `EF_STDERR_OVERFLOW` | 6 | Stderr file exceeded max size |
| `EF_STDPIPE_TIMEOUT` | 7 | Pipe I/O timed out |
| `EF_TIME_LIMIT_HIT_POST` | 8 | Time limit exceeded (detected post-exit) |
| `EF_MEMORY_LIMIT_HIT_POST` | 9 | Memory limit exceeded (detected post-exit) |
| `EF_PROCESS_LIMIT_HIT` | 10 | Active process count exceeded |
| `EF_PROCESS_LIMIT_HIT_POST` | 11 | Process limit exceeded (detected post-exit) |
| `EF_STOPPED` | 12 | Process stopped by signal (Linux) |
| `EF_KILLED_BY_OTHER` | 13 | Process killed by external signal (Linux) |
| `EF_WALL_TIME_LIMIT_HIT_POST` | 14 | Wall time limit exceeded (detected post-exit) |
| `EF_KERNEL_TIME_LIMIT_HIT` | 15 | Kernel time exceeded soft limit |
| `EF_KERNEL_TIME_LIMIT_HIT_POST` | 16 | Kernel time limit exceeded (detected post-exit) |

### 2.9 Process Termination (Windows)

`loopTerminate()`: Repeatedly calls `TerminateProcess()` followed by `WaitForSingleObject(1 second)` until the process is confirmed dead. This handles cases where a single terminate call isn't immediately effective.

### 2.10 Interactive Execution (Interconnect)

Two subprocesses can be connected via pipes for interactive problems:

```
Interconnect(s1, s2, recordInput, recordOutput, interactionLogFile, recorder):
  Create two recording pipes:
    pipe1: s2.stdout → [record] → s1.stdin   (interactor output → program input)
    pipe2: s1.stdout → [record] → s2.stdin   (program output → interactor input)

  Assign pipe endpoints to subprocess redirects:
    s1.StdIn  = read end of pipe1
    s1.StdOut = write end of pipe2
    s2.StdIn  = read end of pipe2
    s2.StdOut = write end of pipe1
```

**Recording pipes**: When recording is requested, each pipe becomes a chain: `writer → tee(recorder, destination) → reader`. Data passes through a goroutine that copies from the read end of one pipe to the write end of another, optionally tee-ing to a recording file.

**Interaction log format**: Lines are prefixed with `"< "` (direction 0, input to program) or `"> "` (direction 1, output from program). When direction changes mid-line (without a newline), `"\n~"` is inserted to mark the switch. The log is thread-safe (mutex-protected) and uses buffered I/O.

### 2.11 Login / User Impersonation

#### Windows

```
LoginInfo.Prepare():
  1. LogonUser(username, ".", password, LOGON32_LOGON_INTERACTIVE)  → HUser token
  2. LoadUserProfile(HUser, username)  → HProfile handle
  3. Register finalizer: UnloadUserProfile + CloseHandle on GC
```

The user token is passed to `CreateProcessWithLogonW` or `CreateProcessAsUser`.

#### Linux

```
NewLoginInfo(username, password):
  1. user.Lookup(username)  → system user
  2. Extract numeric UID
  # Password is ignored on Linux
```

### 2.12 DLL Injection (Windows)

When `PlatformOptions.InjectDLL` contains entries:

1. Get `LoadLibraryW` function address from platform global data
2. Detect target binary type (32-bit vs 64-bit) via `GetBinaryType`
3. Select matching `LoadLibraryW` address for the target architecture
4. For each DLL path:
   a. `VirtualAllocEx` — allocate memory in target process
   b. `WriteProcessMemory` — write DLL path (UTF-16) into allocated memory
   c. `CreateRemoteThread` — call `LoadLibraryW(allocated_path)` in target
   d. `WaitForSingleObject` — wait for DLL load to complete

---

## 3. Protobuf Protocol

All RPC communication uses Protocol Buffers (proto3). Package: `contester.proto`.

### 3.1 Data Types

#### Blob — Compressed Data Container

```protobuf
message Blob {
  message CompressionInfo {
    enum CompressionType {
      METHOD_NONE = 0;
      METHOD_ZLIB = 1;
    }
    CompressionType method = 1;
    uint32 original_size = 2;
  }
  bytes data = 1;
  CompressionInfo compression = 2;
  bytes sha1 = 3;
}
```

Helper methods:
- `Blob.Reader()` → `io.Reader` that decompresses if needed
- `Blob.Bytes()` → decompressed byte slice
- `NewBlob(data)` → creates Blob with zlib compression and SHA-1 hash

#### FileBlob — Named File Data

```protobuf
message FileBlob {
  string name = 1;
  Blob data = 2;
}
```

#### Module — Named Typed Blob

```protobuf
message Module {
  string type = 1;
  Blob data = 2;
  string name = 3;
}
```

### 3.2 Execution Messages

#### RedirectParameters

```protobuf
message RedirectParameters {
  string filename = 1;              // File path for file-based redirect
  bool memory = 2;                  // If true, capture to memory buffer
  Blob buffer = 3;                  // Input data for memory-based stdin
  string remote_filename = 4;       // URL for remote stream
  string remote_authorization_token = 5;
}
```

#### LocalExecutionParameters

```protobuf
message LocalExecutionParameters {
  string application_name = 1;
  string command_line = 2;
  string current_directory = 3;
  uint64 time_limit_micros = 4;         // User+kernel time limit
  uint64 memory_limit = 5;              // Memory limit in bytes
  bool check_idleness = 6;
  LocalEnvironment environment = 7;
  bool restrict_ui = 8;
  bool no_job = 9;
  uint32 process_limit = 10;
  RedirectParameters std_in = 12;
  RedirectParameters std_out = 13;
  RedirectParameters std_err = 14;
  repeated string command_line_parameters = 16;
  string sandbox_id = 17;               // e.g. "%0.C" or "%0.R"
  bool join_stdout_stderr = 18;
  uint64 kernel_time_limit_micros = 19;
  uint64 wall_time_limit_micros = 20;
}
```

#### LocalExecutionResult

```protobuf
message LocalExecutionResult {
  ExecutionResultFlags flags = 1;
  ExecutionResultTime time = 2;
  uint64 memory = 3;               // Peak memory in bytes
  uint32 return_code = 4;          // Process exit code
  Blob std_out = 5;                // Captured stdout
  Blob std_err = 6;                // Captured stderr
  uint64 total_processes = 7;
  int32 kill_signal = 8;           // Linux: signal that killed process
  int32 stop_signal = 9;           // Linux: signal that stopped process
  string error = 10;               // Error message if execution setup failed
}
```

#### ExecutionResultFlags

```protobuf
message ExecutionResultFlags {
  bool killed = 1;
  bool time_limit_hit = 2;
  bool memory_limit_hit = 3;
  bool inactive = 4;
  bool stdout_overflow = 6;
  bool stderr_overflow = 7;
  bool stdpipe_timeout = 8;
  bool time_limit_hit_post = 9;
  bool memory_limit_hit_post = 10;
  bool process_limit_hit = 11;
  bool stopped_by_signal = 12;
  bool killed_by_signal = 13;
  bool kernel_time_limit_hit = 14;
  bool kernel_time_limit_hit_post = 15;
  bool wall_time_limit_hit = 16;
}
```

#### ExecutionResultTime

```protobuf
message ExecutionResultTime {
  uint64 user_time_micros = 1;
  uint64 kernel_time_micros = 2;
  uint64 wall_time_micros = 3;
}
```

#### LocalExecuteConnected — Interactive Execution

```protobuf
message LocalExecuteConnected {
  LocalExecutionParameters first = 1;   // Program
  LocalExecutionParameters second = 2;  // Interactor
}

message LocalExecuteConnectedResult {
  LocalExecutionResult first = 1;
  LocalExecutionResult second = 2;
}
```

### 3.3 Identification Messages

```protobuf
message IdentifyRequest {
  string contester_id = 1;
}

message SandboxLocations {
  string compile = 1;
  string run = 2;
}

message IdentifyResponse {
  string invoker_id = 1;
  repeated SandboxLocations sandboxes = 2;
  LocalEnvironment environment = 3;
  string platform = 4;                  // "windows" or "linux"
  string path_separator = 5;            // "\" or "/"
  repeated string disks = 6;            // Windows drive letters
  repeated string programFiles = 7;     // Program Files directories
}
```

### 3.4 File Operation Messages

```protobuf
message FileStat {
  string name = 1;
  bool is_directory = 2;
  uint64 size = 3;
  string checksum = 4;           // "sha1:<hex>" format
}

message StatRequest {
  repeated string name = 1;
  string sandbox_id = 2;
  bool expand = 3;               // Treat names as glob patterns
  bool calculate_checksum = 4;   // Compute SHA-1
}

message FileStats {
  repeated FileStat entries = 1;
}

message GetRequest {
  string name = 1;
}

message ClearSandboxRequest {
  string sandbox = 1;
}
```

### 3.5 Remote Storage Messages

```protobuf
message CopyOperation {
  string local_file_name = 1;
  string remote_location = 2;
  bool upload = 3;                      // true=upload to remote, false=download
  string checksum = 4;                  // Expected SHA-1
  string module_type = 5;
  string authorization_token = 6;
}

message CopyOperations {
  repeated CopyOperation entries = 1;
  string sandbox_id = 2;
}
```

### 3.6 Compilation Messages

```protobuf
message Compilation {
  enum Code {
    Unknown = 0;
    Success = 1;
    Failure = 2;
  }
  message Result {
    string step_name = 1;
    LocalExecution execution = 2;
    bool failure = 3;
  }
  bool failure = 1;
  repeated Result result_steps = 2;
}
```

### 3.7 Environment Messages

```protobuf
message LocalEnvironment {
  message Variable {
    string name = 1;
    string value = 2;
    bool expand = 3;      // Expand environment variables in value
  }
  bool empty = 1;          // If true, start with empty environment
  repeated Variable variable = 2;
}
```

---

## 4. Runner Service (Reverse RPC)

### 4.1 Architecture

The `runner` binary implements a **reverse RPC** pattern:

1. Runner (client) connects via TCP to the dispatcher (server)
2. Runner registers its `Contester` service with Go's `net/rpc`
3. Once connected, the runner serves RPC requests on that connection using `rpc4go` codec
4. When the connection drops, the runner reconnects after a 5-second delay
5. Connection parameters: 1-minute dial timeout, 10-second TCP keepalive

```
Runner                              Dispatcher
  |---- TCP connect ----------------->|
  |<--- RPC request (Identify) -------|
  |---- RPC response (capabilities) ->|
  |<--- RPC request (Execute) --------|
  |---- RPC response (result) ------->|
  |           ...                     |
  |<--- connection drops ------------->|
  |---- reconnect (5s delay) -------->|
```

### 4.2 Configuration

TOML config file (default: `server.toml`):

```toml
Server = "dispatcher.example.com:5000"    # Dispatcher address
Path = "/var/lib/contester/sandboxes"     # Base path for sandbox directories
SandboxCount = 4                          # Number of sandbox pairs (auto on Windows)
Passwords = "pwd1 pwd2 pwd3 pwd4"         # Space-separated passwords for sandbox users
```

CLI flags:
- `-l/--logfile` — Log file path (default: `server.log`)
- `-c/--config` — Config file path (default: `server.toml`)

### 4.3 Startup Sequence

1. Parse CLI args (Kong)
2. Open log file (append mode, debug level)
3. Create global platform data (`platform.CreateGlobalData(true)`)
4. Load TOML config
5. Create `Contester` service:
   a. Set invoker ID to hostname
   b. Capture current environment
   c. Configure sandboxes (create directories, users, set ACLs)
6. Register service with `rpc.DefaultServer`
7. Enter connection loop

### 4.4 RPC Methods

All methods are registered under the `Contester` service name (e.g., `Contester.Identify`).

#### `Identify(IdentifyRequest) → IdentifyResponse`

Returns runner capabilities and configuration:
- Invoker ID (hostname)
- List of sandbox pairs with their compile/run paths
- Full environment variable dump
- Platform ("windows" or "linux")
- Path separator
- Available disk drives (Windows)
- Program Files directories (Windows)

#### `LocalExecute(LocalExecutionParameters) → LocalExecutionResult`

Execute a single process:
1. Resolve sandbox from `sandbox_id` or `current_directory`
2. Lock sandbox mutex (exclusive)
3. `chmod` the application if needed (Linux)
4. Build `Subprocess` from parameters
5. Execute and collect result
6. Convert result to protobuf response

#### `LocalExecuteConnected(LocalExecuteConnected) → LocalExecuteConnectedResult`

Execute two interconnected processes (interactive problem):
1. Resolve both sandboxes
2. Lock both sandbox mutexes
3. `chmod` both applications if needed
4. Build both `Subprocess` configs (without I/O redirects — pipes will be set up)
5. Call `Interconnect()` to connect them via pipes
6. Execute both concurrently via goroutines
7. Wait for both to complete
8. Return both results

**Note**: No deadlock prevention is implemented for the case where both processes resolve to the same sandbox — the code locks `firstSandbox.Mutex` then `secondSandbox.Mutex` sequentially.

#### `Put(FileBlob) → FileStat`

Write a file to a sandbox:
1. Resolve path (sandbox ID prefix or absolute)
2. Lock sandbox mutex
3. Create file, decompress blob, write contents
4. Return file metadata with SHA-1 checksum

#### `Get(GetRequest) → FileBlob`

Read a file from a sandbox:
1. Resolve path
2. Lock sandbox mutex
3. Read file, compress with zlib, compute SHA-1
4. Return as FileBlob

#### `Stat(StatRequest) → FileStats`

Query file metadata:
1. Resolve paths (with optional glob expansion when `expand=true`)
2. For each path: get name, is_directory, size
3. Optionally compute SHA-1 checksum
4. Return list of FileStat entries

#### `GridfsCopy(CopyOperations) → FileStats`

Transfer files between runner and remote storage:
1. For each CopyOperation:
   - Resolve local path via sandbox ID
   - If `upload=true`: HTTP PUT to `remote_location` with auth token
   - If `upload=false`: HTTP GET from `remote_location` to local file
2. Return FileStat for each transferred file

#### `Clear(ClearSandboxRequest) → EmptyMessage`

Wipe sandbox contents:
1. Resolve sandbox path
2. Attempt to remove all contents recursively
3. Retry up to 10 times with 500ms delay between attempts (handles locked files)

---

## 5. Sandbox System

### 5.1 Structure

```
Sandbox:
  Path: string              # Filesystem path
  Mutex: sync.RWMutex       # Exclusive lock during operations
  Login: *LoginInfo         # User credentials for processes in this sandbox

SandboxPair:
  Compile: *Sandbox         # For compilation steps
  Run: *Sandbox             # For execution steps (more restricted)
```

### 5.2 Directory Layout

```
<base_path>/
  0/
    C/    # Sandbox 0 compile directory
    R/    # Sandbox 0 run directory
  1/
    C/    # Sandbox 1 compile directory
    R/    # Sandbox 1 run directory
  ...
```

### 5.3 Sandbox ID Format

Sandbox IDs are used in RPC requests to refer to specific sandboxes:

Format: `%<index>.<variant>` where:
- `index`: 0-based integer (sandbox pair index)
- `variant`: `C` (compile) or `R` (run)

Example: `%0.R` refers to sandbox pair 0, run directory.

Paths can be prefixed with sandbox IDs: `%0.C/program.exe` resolves to `<base_path>/0/C/program.exe`.

### 5.4 User Management

#### Windows

- Each run sandbox gets a dedicated local user account: `tester0`, `tester1`, ...
- Passwords are either from config or auto-generated (8 random chars from `a-zA-Z0-9`)
- Sandbox count auto-detected: `(NumCPU / 2) - 1`
- Users created via `NetUserAdd` if they don't exist
- ACLs set via `subinacl.exe`
- On logon failure, passwords are reset via `NetUserSetInfo`

#### Linux

- Compile sandbox: user `compiler` (expected to exist in system)
- Run sandboxes: users `tester0`, `tester1`, ... (expected to exist in system)
- Passwords are `"password" + index` (e.g., `password0`) — ignored by the system
- Directory ownership set via `chown`, permissions `0700`
- Users created via `useradd` with home directory disabled

### 5.5 Sandbox Path Resolution

When resolving a path from an RPC request:
1. If path starts with `%`: parse sandbox ID, resolve relative to sandbox directory
2. If path is absolute: use as-is (allowed for stat/read operations)
3. Relative paths: rejected

---

## 6. runexe CLI

### 6.1 Usage

```
runexe [global-flags] [process-flags] <application> [args...]
```

For interactive mode:
```
runexe [global-flags] [process-flags] <application> [args...] \
  --interactor "[interactor-flags] <interactor-app> [interactor-args...]"
```

### 6.2 Process Flags

| Flag | Description |
|------|-------------|
| `-t <time>` | Time limit (e.g., `1s`, `500ms`, `2000` = 2000ms) |
| `-h <time>` | Wall time limit |
| `-m <memory>` | Memory limit (e.g., `256M`, `1G`, `1048576` bytes) |
| `-d <path>` | Working directory (default: current directory) |
| `-i <file>` | Stdin redirect from file |
| `-o <file>` | Stdout redirect to file |
| `-e <file>` | Stderr redirect to file |
| `-os <size>` | Max stdout file size |
| `-es <size>` | Max stderr file size |
| `-u` | Join stderr into stdout |
| `-l <user>` | Login username |
| `-p <pass>` | Login password |
| `-j <dll>` | DLL to inject (Windows) |
| `-a <mask>` | CPU affinity mask |
| `-D <KEY=VALUE>` | Set environment variable (repeatable) |
| `--envfile <file>` | Load environment from file (one `KEY=VALUE` per line) |
| `-z` | Trusted mode (disable UI restrictions) |
| `--no-idleness-check` | Disable idleness detection |
| `--no-job` | Don't use job object (Windows) |
| `--process-limit <n>` | Max child processes |

### 6.3 Global Flags

| Flag | Description |
|------|-------------|
| `--xml` | Output results as XML |
| `--interactor "<flags>"` | Interactor command line (quoted, re-parsed) |
| `--show-kernel-mode-time` | Show kernel time in text output |
| `-x` | Exit with the program's exit code |
| `--logfile <file>` | Log file for debug output |
| `--ri <file>` | Record program input to file |
| `--ro <file>` | Record program output to file |
| `--interaction-log <file>` | Record interaction log to file |

### 6.4 Verdict Determination

The verdict is determined from `SubprocessResult` in priority order:

1. **OUTPUT_LIMIT_EXCEEDED**: `OutputLimitExceeded` or `ErrorLimitExceeded`
2. **SUCCEEDED**: `SuccessCode == 0`
3. **SECURITY_VIOLATION**: `EF_PROCESS_LIMIT_HIT` or `EF_PROCESS_LIMIT_HIT_POST`
4. **IDLENESS_LIMIT_EXCEEDED**: `EF_INACTIVE` or `EF_WALL_TIME_LIMIT_HIT`
5. **TIME_LIMIT_EXCEEDED**: Any of `EF_TIME_LIMIT_HIT`, `EF_TIME_LIMIT_HIT_POST`, `EF_KERNEL_TIME_LIMIT_HIT`, `EF_KERNEL_TIME_LIMIT_HIT_POST`
6. **MEMORY_LIMIT_EXCEEDED**: `EF_MEMORY_LIMIT_HIT` or `EF_MEMORY_LIMIT_HIT_POST`
7. **CRASHED**: Any other non-zero `SuccessCode` or execution error (user error → CRASHED, system error → FAILED)

### 6.5 Output Formats

#### Text Output

```
Program successfully terminated
  exit code:    0
  time consumed: 0.15 sec
  time passed:  0.20 sec
  peak memory:  4096000 bytes
```

On time limit exceeded:
```
Time limit exceeded
Program failed to terminate within 1.00 sec
  time consumed: 1.00 of 1.00 sec
  time passed:  1.25 sec
  peak memory:  4096000 bytes
```

#### XML Output

```xml
<?xml version="1.1" encoding="UTF-8"?>
<invocationResults>
  <invocationResult id="program">
    <invocationVerdict>SUCCEEDED</invocationVerdict>
    <exitCode>0</exitCode>
    <processorUserModeTime>150</processorUserModeTime>
    <processorKernelModeTime>10</processorKernelModeTime>
    <passedTime>200</passedTime>
    <consumedMemory>4096000</consumedMemory>
  </invocationResult>
</invocationResults>
```

Times in XML are in **milliseconds**. Memory is in **bytes**.

For interactive mode, both `program` and `interactor` results are included.

### 6.6 Interactor Parsing

The `--interactor` value is a full command line string that is parsed using Windows command-line conventions (handling of backslashes, quotes, spaces) via `commandLineToArgv()`. The resulting args are fed through the same flag parser as the main program.

---

## 7. Remote Storage

### 7.1 Backend Interface

```go
type Backend interface {
    Copy(ctx context.Context, localName, remoteName string, toRemote bool,
         checksum, moduleType, authToken string) (*FileStat, error)
    ReadRemote(ctx context.Context, name, authToken string) (*RemoteFile, error)
}
```

### 7.2 Weed Filer Backend

HTTP-based storage using SeaweedFS Filer:

**Upload (PUT)**:
- URL: `remote_location` from CopyOperation
- Headers:
  - `X-FS-Module-Type: <moduleType>`
  - `Authorization: Bearer <authToken>`
  - `Digest: sha-1=<base64(sha1)>` (if checksum provided)
- Body: raw file contents
- Timeout: 1 minute

**Download (GET)**:
- URL: `remote_location`
- Headers: `Authorization: Bearer <authToken>`
- Response written to local file
- Timeout: 1 minute

### 7.3 Problem Manifests

The `storage` package also handles problem metadata:

```go
ProblemManifest:
  Key: string              // MongoDB _id (grid prefix)
  Id: string               // Problem identifier (URL or direct:path)
  Revision: int
  TestCount: int
  TimeLimitMicros: int64
  MemoryLimit: int64
  TesterName: string       // Checker program name
  InteractorName: string   // Interactor program name (for interactive problems)
  CombinedHash: string     // Aggregate hash of all problem files
```

Grid prefix generation from problem IDs:
- `polygon/https/codeforces.com/problems/123A` → `problem/polygon/https/codeforces.com/problems/123A/`
- `direct:/path/to/problem` → `problem/direct/path/to/problem/`

---

## 8. Platform Abstraction Layer

### 8.1 GlobalData

Platform-wide state initialized at startup.

**Windows**:
```go
GlobalData:
  desktop: *ContesterDesktop    // Lazy-initialized
  desktopErr: error
  loadLibraryW: uintptr         // LoadLibraryW function pointer
  archData: archGlobalData      // Architecture-specific (32/64-bit)
  mu: sync.Mutex                // Protects lazy initialization
```

**Linux**: Empty struct, all methods are no-ops.

### 8.2 Desktop Isolation (Windows)

On pre-Windows 8 systems, processes run in an isolated desktop:

1. `CreateWindowStation("ContesterWinSta")` — new window station
2. `CreateDesktop("ContesterDesktop", ...)` — new desktop in that station
3. Process's `STARTUPINFO.Desktop` set to `"ContesterWinSta\ContesterDesktop"`

This prevents the sandboxed process from interacting with the user's desktop.

### 8.3 Win32 API Wrappers

The `win32` package provides Go bindings for:

| DLL | Functions |
|-----|-----------|
| advapi32 | `LogonUserW`, `CreateProcessAsUserW`, ACL operations |
| kernel32 | `CreateJobObjectW`, `SetInformationJobObject`, `VirtualAllocEx`, `WriteProcessMemory`, `CreateRemoteThread`, `SetProcessAffinityMask`, `GetBinaryTypeW` |
| psapi | `GetProcessMemoryInfo` |
| userenv | `LoadUserProfileW`, `UnloadUserProfile` |
| user32 | `CreateWindowStationW`, `CreateDesktopW`, `GetProcessWindowStation` |
| netapi32 | `NetUserAdd`, `NetUserEnum`, `NetUserSetInfo`, `NetLocalGroupAddMembers` |

---

## 9. Build System

### 9.1 Module

```
module: github.com/contester/runlib
go: 1.20
```

### 9.2 Key Dependencies

| Module | Purpose |
|--------|---------|
| `github.com/contester/rpc4/rpc4go` | Custom RPC codec for reverse-RPC protocol |
| `github.com/sirupsen/logrus` | Structured logging |
| `golang.org/x/sys` | System call access |
| `google.golang.org/protobuf` | Protocol Buffers runtime |
| `github.com/BurntSushi/toml` | TOML config parsing |
| `github.com/alecthomas/kong` | CLI argument parsing (runner) |

### 9.3 CI Build

GitHub Actions workflow builds for both `ubuntu-latest` and `windows-latest` with Go 1.21.3.

Build command pattern:
```bash
go build -o <binary>.exe -a \
  -ldflags "-X main.version=<timestamp>-<git_sha> -X main.buildid=<run_number>" \
  ./<package>
```

Artifacts uploaded: `runexe.exe`, `runner.exe`.

### 9.4 Protobuf Generation

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  Blobs.proto Contester.proto Execution.proto Local.proto
```

---

## 10. Key Design Decisions and Invariants

1. **Subprocess struct is immutable during execution**. All mutable state lives in `SubprocessData`.

2. **Processes are always created suspended** (`CREATE_SUSPENDED` on Windows, frozen clone on Linux). This ensures the parent can set up job objects, cgroups, and monitoring before the child runs.

3. **Soft vs hard limits**: Soft limits are checked by the monitoring loop every `TimeQuantum`. Hard limits are enforced by the OS (job objects, cgroups). Hard time limit is set to `soft + 1s` to give the monitor a chance to detect and report the violation.

4. **Post-exit limit checking**: After the process exits, limits are checked one final time using `SetPostLimits()`. This catches cases where the process exceeded limits in its final moments before the monitoring loop could detect it.

5. **Memory measurement on Windows**: With job objects, `PeakJobMemoryUsed` is used because it accounts for all processes in the job. Without job objects, `max(PeakPagefileUsage, PrivateUsage)` is used as a best-effort single-process measurement.

6. **Sandbox mutex**: Each sandbox has an exclusive lock. Only one operation can use a sandbox at a time. This prevents concurrent executions from interfering with each other's files.

7. **Reverse RPC**: The runner connects to the dispatcher (not the other way around). This simplifies network configuration — runners can be behind NATs/firewalls. The runner reconnects automatically on connection failure.

8. **Blob compression**: All file data transferred via protobuf uses zlib compression and SHA-1 integrity verification.

9. **Interactive execution**: Both processes run concurrently in separate goroutines. Pipe connectivity is established before either process is unfrozen. There is no timeout on pipe operations themselves (they rely on process time limits to terminate).

10. **Idleness detection requires both** no CPU progress for 6+ quanta AND wall time exceeding the time limit. This prevents false positives from legitimate I/O waits that complete within the time limit.

11. **OS thread locking on Linux**: Required because ptrace operations must happen on the same OS thread that initiated the trace.
