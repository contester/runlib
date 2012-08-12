package sub32

import (
  "syscall"
  "unsafe"
)

type Subprocess struct {
  ApplicationName string
  CommandLine string
  CurrentDirectory string
  Environment *[]string

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

func (sub *Subprocess) LaunchProcess() (processInfo *syscall.ProcessInformation, err error) {
  si := &syscall.StartupInfo{}
  si.Cb = uint32(unsafe.Sizeof(*si))

  pi := &syscall.ProcessInformation{}
  
  e := syscall.CreateProcess(
      StringPtrToUTF16Ptr(&sub.ApplicationName),
      StringPtrToUTF16Ptr(&sub.CommandLine),
      nil,
      nil,
      true,
      CREATE_NEW_PROCESS_GROUP | CREATE_NEW_CONSOLE | CREATE_SUSPENDED | syscall.CREATE_UNICODE_ENVIRONMENT | CREATE_BREAKAWAY_FROM_JOB,
      ListToEnvironmentBlock(sub.Environment),
      StringPtrToUTF16Ptr(&sub.CurrentDirectory),
      si,
      pi);
  if (e != nil) {
    return nil, e
  }

  return pi, nil
}