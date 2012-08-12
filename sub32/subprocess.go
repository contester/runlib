package sub32

import (
  "syscall"
  "unsafe"
)

type Subprocess struct {
  ApplicationName *string
  CommandLine *string
  CurrentDirectory *string
  Environment *[]string

  Username *string
  Password *string

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

func (sub *Subprocess) Launch() (processInfo *syscall.ProcessInformation, err error) {
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
    return nil, e
  }

  return pi, nil
}