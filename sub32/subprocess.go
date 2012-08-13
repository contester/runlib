package sub32

import (
  "bytes"
  "io"
  "os"
  "syscall"
  "unsafe"
)

const (
  EF_INACTIVE = (1 << 0)
  EF_TIME_LIMIT_HIT = (1 << 1)
  EF_TIME_LIMIT_HARD = (1 << 2)
  EF_MEMORY_LIMIT_HIT = (1 << 3)
  EF_KILLED = (1 << 4)
  EF_STDOUT_OVERFLOW = (1 << 5)
  EF_STDERR_OVERFLOW = (1 << 6)
  EF_STDPIPE_TIMEOUT = (1 << 7)
  EF_TIME_LIMIT_HIT_POST = (1 << 8)
  EF_MEMORY_LIMIT_HIT_POST = (1 << 9)
  EF_PROCESS_LIMIT_HIT = (1 << 10)
  EF_PROCESS_LIMIT_HIT_POST = (1 << 11)
)

type SubprocessOutputRedirect struct {
  ToMemory bool
  ToFile *string

  buffer *bytes.Buffer
  reader *os.File
  writer *os.File
}

type Subprocess struct {
  ApplicationName *string
  CommandLine *string
  CurrentDirectory *string
  Environment *[]string

  Username *string
  Password *string

  NoJob bool
  RestrictUi bool
  ProcessLimit uint32
  CheckIdleness bool

  TimeLimit uint64
  HardTimeLimit uint64
  MemoryLimit uint64
  HardMemoryLimit uint64
  TimeQuantum uint32

  StdOut *SubprocessOutputRedirect

  hProcess syscall.Handle
  hThread syscall.Handle

  bufferFunctions []func() error
  bufferChan chan error

  /*
  HANDLE hJob, hProcess, bhThread, hUser,
    hProfile, hWindowStation, hDesktop, hThread;

  WCHAR
    *wApplicationName,
    *wCommandLine,
    *wCurrentDirectory,
    *wEnvironment,

    *wUsername,
    *wPassword,
    *wDomain,

    *wInjectDll;

  bool NoJob;
  bool RestrictUI;
  unsigned int ProcessLimit;

  struct SubprocessErrorEntry ErrorEntries[32];
  unsigned int Errors;
  CRITICAL_SECTION csError;

  bool CheckIdleness;
  uint64_t TimeLimit;
  uint64_t HardTimeLimit;
  uint64_t MemoryLimit;
  uint64_t HardMemoryLimit;
  uint64_t TimeQuantum;

  struct SubprocessResult srResult;
  struct RedirectParameters * rp[REDIRECT_LAST];

  SubprocessCbFunc cb;
  void * cbarg;

  void* (*mallocfunc)(size_t);
  void* (*reallocfunc)(void*, size_t);
  void (*freefunc)(void*);
  */
}

type SubprocessResult struct {
  SuccessCode uint32
  ExitCode uint32
  UserTime uint64
  KernelTime uint64
  WallTime uint64
  PeakMemory uint64
  TotalProcesses uint64

  Output *bytes.Buffer
}

func SubprocessCreate() *Subprocess {
  result := &Subprocess{}
  result.TimeQuantum = 1000

  return result
}

func (sub *Subprocess) Launch() (err error) {
  _ = sub.SetupRedirects()

  si := &syscall.StartupInfo{}
  si.Cb = uint32(unsafe.Sizeof(*si))
  si.Flags = STARTF_FORCEOFFFEEDBACK | syscall.STARTF_USESHOWWINDOW;
  si.ShowWindow = syscall.SW_SHOWMINNOACTIVE
  if sub.StdOut != nil {
    si.StdOutput = syscall.Handle(sub.StdOut.writer.Fd())
  }


  pi := &syscall.ProcessInformation{}

  applicationName := StringPtrToUTF16Ptr(sub.ApplicationName)
  commandLine := StringPtrToUTF16Ptr(sub.CommandLine)
  environment := ListToEnvironmentBlock(sub.Environment)
  currentDirectory := StringPtrToUTF16Ptr(sub.CurrentDirectory)

  var e error

  if (sub.Username != nil) {
    e = CreateProcessWithLogonW(
      StringPtrToUTF16Ptr(sub.Username),
      syscall.StringToUTF16Ptr("."),
      StringPtrToUTF16Ptr(sub.Password),
      LOGON_WITH_PROFILE,
      applicationName,
      commandLine,
      CREATE_SUSPENDED | syscall.CREATE_UNICODE_ENVIRONMENT,
      environment,
      currentDirectory,
      si,
      pi);
  } else {  
    e = syscall.CreateProcess(
      applicationName,
      commandLine,
      nil,
      nil,
      true,
      CREATE_NEW_PROCESS_GROUP | CREATE_NEW_CONSOLE | CREATE_SUSPENDED | syscall.CREATE_UNICODE_ENVIRONMENT | CREATE_BREAKAWAY_FROM_JOB,
      environment,
      currentDirectory,
      si,
      pi);
  }

  if (e != nil) {
    return e
  }

  sub.hProcess = pi.Process
  sub.hThread = pi.Thread

  sub.bufferChan = make(chan error, len(sub.bufferFunctions))

  for _, fn := range sub.bufferFunctions {
    go func(fn func() error) {
      sub.bufferChan <- fn()
    }(fn)
  }

  return nil
}

func FiletimeToUint64(ft *syscall.Filetime) uint64 {
  return uint64(ft.HighDateTime)<<32 + uint64(ft.LowDateTime)
}

func UpdateProcessTimes(process syscall.Handle, result *SubprocessResult, finished bool) error {
  creation := &syscall.Filetime{}
  end := &syscall.Filetime{}
  user := &syscall.Filetime{}
  kernel := &syscall.Filetime{}

  err := syscall.GetProcessTimes(process, creation, end, kernel, user)
  if err != nil {
    return err
  }

  if !finished {
    syscall.GetSystemTimeAsFileTime(end)
  }

  result.WallTime = (FiletimeToUint64(end) / 10) - (FiletimeToUint64(creation) / 10)
  result.UserTime = FiletimeToUint64(user) / 10
  result.KernelTime = FiletimeToUint64(kernel) / 10

  return nil
}

func GetProcessMemoryUsage(process syscall.Handle) uint32 {
  pmc, err := GetProcessMemoryInfo(process)
  if err != nil {
    return 0
  }

  if pmc.PeakPagefileUsage > pmc.PrivateUsage {
    return pmc.PeakPagefileUsage
  }
  return pmc.PrivateUsage
}

func UpdateProcessMemory(process syscall.Handle, result *SubprocessResult) {
  result.PeakMemory = uint64(GetProcessMemoryUsage(process))
}

func (sub *Subprocess) SetupRedirects() error {
  if sub.StdOut != nil {
    sub.StdOut.buffer = &bytes.Buffer{}
    var err error
    sub.StdOut.reader, sub.StdOut.writer, err = os.Pipe()
    if err != nil {
      return err
    }
    sub.bufferFunctions = append(sub.bufferFunctions, func() error {
        _, err := io.Copy(sub.StdOut.buffer, sub.StdOut.reader)
        return err
      })
  }
  return nil
}

func (sub *Subprocess) BottomHalf(sig chan *SubprocessResult) {
  result := &SubprocessResult{}
  var waitResult uint32
  waitResult = syscall.WAIT_TIMEOUT
  var ttLast uint64
  ttLast = 0

  for result.SuccessCode == 0 && waitResult == syscall.WAIT_TIMEOUT {
    waitResult, _ = syscall.WaitForSingleObject(sub.hProcess, sub.TimeQuantum)
    if waitResult != syscall.WAIT_TIMEOUT {
      break
    }

    _ = UpdateProcessTimes(sub.hProcess, result, false)
    ttLastNew := result.KernelTime + result.UserTime

    if sub.CheckIdleness && (ttLast == ttLastNew) {
      result.SuccessCode |= EF_INACTIVE
    }

    if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
      result.SuccessCode |= EF_TIME_LIMIT_HIT
    }

    if (sub.HardTimeLimit > 0) && (result.WallTime > sub.HardTimeLimit) {
      result.SuccessCode |= EF_TIME_LIMIT_HARD
    }

    ttLast = ttLastNew

    if (sub.MemoryLimit > 0) {
      UpdateProcessMemory(sub.hProcess, result)
      if result.PeakMemory > sub.MemoryLimit {
        result.SuccessCode |= EF_MEMORY_LIMIT_HIT
      }
    }
  }

  switch waitResult {
    case syscall.WAIT_OBJECT_0:
      _ = syscall.GetExitCodeProcess(sub.hProcess, &result.ExitCode)

    case syscall.WAIT_TIMEOUT:
      for waitResult == syscall.WAIT_TIMEOUT {
        syscall.TerminateProcess(sub.hProcess, 0)
        waitResult, _ = syscall.WaitForSingleObject(sub.hProcess, 100)
      }
  }

  _ = UpdateProcessTimes(sub.hProcess, result, true)
  UpdateProcessMemory(sub.hProcess, result)

  syscall.CloseHandle(sub.hProcess)

  if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
    result.SuccessCode |= EF_TIME_LIMIT_HIT_POST
  }

  if (sub.MemoryLimit > 0) && (result.PeakMemory > sub.MemoryLimit) {
    result.SuccessCode |= EF_MEMORY_LIMIT_HIT_POST
  }

  for _ = range sub.bufferFunctions {
    <-sub.bufferChan
  }

  if sub.StdOut != nil {
    result.Output = sub.StdOut.buffer
  }

  sig <- result
}

func (sub *Subprocess) Start() (chan *SubprocessResult, error) {
  err := sub.Launch()
  if (err != nil) {
    return nil, err
  }
  
  ResumeThread(sub.hThread)
  syscall.CloseHandle(sub.hThread)

  sig := make(chan *SubprocessResult)
  go sub.BottomHalf(sig)
  return sig, nil
}