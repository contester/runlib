package sub32

type Subprocess struct {
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