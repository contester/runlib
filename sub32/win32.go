package sub32

import (
  "syscall"
  "unsafe"
)

var (
  advapi32 = syscall.NewLazyDLL("advapi32.dll")
  kernel32 = syscall.NewLazyDLL("kernel32.dll")

  procCreateProcessWithLogonW = advapi32.NewProc("CreateProcessWithLogonW")
  procCreateProcessW = kernel32.NewProc("CreateProcessW")
)

const (
  CREATE_BREAKAWAY_FROM_JOB = 0x01000000
  CREATE_NEW_CONSOLE = 0x00000010
  CREATE_NEW_PROCESS_GROUP = 0x00000200
  CREATE_SUSPENDED = 0x00000004

  LOGON_WITH_PROFILE = 0x00000001

  STARTF_FORCEOFFFEEDBACK = 0x00000080
)

func StringPtrToUTF16Ptr(src *string) (result *uint16) {
  if src != nil {
    return syscall.StringToUTF16Ptr(*src)
  }
  return nil
}

func ListToEnvironmentBlock(list *[]string) *uint16 {
  if (list == nil) {
    return nil
  }

  size := 1
  for _, v := range *list {
    size += len(syscall.StringToUTF16(v))
  }

  result := make([]uint16, size)

  tail := 0

  for _, v := range *list {
    uline := syscall.StringToUTF16(v)
    copy(result[tail:], uline)
    tail += len(uline)
  }

  result[tail] = 0

  return &result[0]
}

/*
func GetEnvMap() map[string] string {
  result := make(map[string] string)

  for _, line := range syscall.Environ() {
    s := strings.SplitN(line, "=", 2)
    if len(s) == 2 {
      result[s[0]] = s[1]
    }
  }
  return result
}

func getEnvBlock(env Environment) *uint16 {
  if (env == nil) {
    return nil
  }

  return &env.ToBlock()[0]
}
*/

func CreateProcessWithLogonW(
    username *uint16,
    domain *uint16,
    password *uint16,
    logonFlags uint32,
    applicationName *uint16,
    commandLine *uint16,
    creationFlags uint32,
    environment *uint16,
    currentDirectory *uint16,
    startupInfo *syscall.StartupInfo,
    processInformation *syscall.ProcessInformation) (err error) {

  r1, _, e1 := procCreateProcessWithLogonW.Call(
    uintptr(unsafe.Pointer(username)),
    uintptr(unsafe.Pointer(domain)),
    uintptr(unsafe.Pointer(password)),
    uintptr(logonFlags),
    uintptr(unsafe.Pointer(applicationName)),
    uintptr(unsafe.Pointer(commandLine)),
    uintptr(creationFlags),
    uintptr(unsafe.Pointer(environment)), // env
    uintptr(unsafe.Pointer(currentDirectory)),
    uintptr(unsafe.Pointer(startupInfo)),
    uintptr(unsafe.Pointer(processInformation)))

  if int(r1) == 0 {
    return e1
  }

  return nil
}
/*
func CreateProcessWithLogonWWWW(
    username string,
    domain *string,
    password string,
    logonFlags uint32,
    applicationName *string,
    commandLine *string,
    creationFlags uint32,
    environment Environment,
    currentDirectory *string,
    startupInfo *StartupInfo) (processInformation *ProcessInformation, err error) {

  pUsername := syscall.StringToUTF16Ptr(username)
  pDomain := nilOrString(domain)
  pPassword := syscall.StringToUTF16Ptr(password)
  pApplicationName := nilOrString(applicationName)
  pCommandLine := nilOrString(commandLine)
  pCurrentDirectory := nilOrString(currentDirectory)
  
  // si := &syscall.StartupInfo{}
  pi := &syscall.ProcessInformation{}

  // testBlock(getEnvBlock(environment))

  r1, _, e1 := procCreateProcessWithLogonW.Call(
    uintptr(unsafe.Pointer(pUsername)),
    uintptr(unsafe.Pointer(pDomain)),
    uintptr(unsafe.Pointer(pPassword)),
    uintptr(LOGON_WITH_PROFILE),
    uintptr(unsafe.Pointer(pApplicationName)),
    uintptr(unsafe.Pointer(pCommandLine)),
    uintptr(syscall.CREATE_UNICODE_ENVIRONMENT), //creationFlags),
    uintptr(unsafe.Pointer(getEnvBlock(environment))), // env
    uintptr(unsafe.Pointer(pCurrentDirectory)),
    uintptr(0), // si
    uintptr(unsafe.Pointer(pi)))

  if int(r1) == 0 {
    return nil, e1
  }

  return nil, nil
}
    
func CreateProcessWWWW(
    applicationName *string,
    commandLine *string,
    environment Environment,
    currentDirectory *string,
    startupInfo StartupInfo) (processInformation *ProcessInformation, err error) {    

  pApplicationName := nilOrString(applicationName)
  pCommandLine := nilOrString(commandLine)
  pCurrentDirectory := nilOrString(currentDirectory)

  pi := &syscall.ProcessInformation{}
  si := &syscall.StartupInfo{}
  si.Cb = uint32(unsafe.Sizeof(*si))
  // si.Flags = syscall.STARTF_USESTDHANDLES

  r1, _, e1 := procCreateProcessW.Call(
    uintptr(unsafe.Pointer(pApplicationName)),
    uintptr(unsafe.Pointer(pCommandLine)),
    uintptr(0),
    uintptr(0),
    uintptr(1),
    uintptr(CREATE_NEW_PROCESS_GROUP | CREATE_NEW_CONSOLE | syscall.CREATE_UNICODE_ENVIRONMENT | CREATE_BREAKAWAY_FROM_JOB),
    uintptr(unsafe.Pointer(getEnvBlock(environment))), // env
    uintptr(unsafe.Pointer(pCurrentDirectory)),
    uintptr(unsafe.Pointer(si)),
    uintptr(unsafe.Pointer(pi)))

  if int(r1) == 0 {
    return nil, e1
  }

  return nil, nil
}    
    
*/