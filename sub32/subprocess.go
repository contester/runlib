package sub32

import (
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
  TimeQuantum uint64

  hProcess syscall.Handle
  hThread syscall.Handle

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
}

func (sub *Subprocess) Launch() (err error) {
  si := &syscall.StartupInfo{}
  si.Cb = uint32(unsafe.Sizeof(*si))
  si.Flags = STARTF_FORCEOFFFEEDBACK | syscall.STARTF_USESHOWWINDOW;
  si.ShowWindow = syscall.SW_SHOWMINNOACTIVE
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

  return nil
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

  result.WallTime = uint64((end.Nanoseconds() / 1000) - (creation.Nanoseconds() / 1000))
  result.UserTime = uint64(user.Nanoseconds() / 1000)
  result.KernelTime = uint64(kernel.Nanoseconds() / 1000)
}

func (sub *Subprocess) BottomHalf(sig chan *SubprocessResult) {
  result := &SubprocessResult{}
  waitResult := 0
  var ttLast uint64
  ttLast = 0

  for ; ;  {
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
  }
  
}

func (sub *Subprocess) Start() error {
  err := sub.Launch()
  if (err != nil) {
    return err
  }
  
  ResumeThread(sub.hThread)
  syscall.CloseHandle(sub.hThread)

  return nil
}