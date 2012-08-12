package sub32

import (
  "syscall"
  "unsafe"
)

var (
  advapi32 = syscall.NewLazyDLL("advapi32.dll")

  procCreateProcessWithLogonW = advapi32.NewProc("CreateProcessWithLogonW")
)

type Environment interface {}
type StartupInfo interface {}
type ProcessInformation interface {}

func nilOrString(src *string) (result *uint16) {
  if src != nil {
    return syscall.StringToUTF16Ptr(*src)
  }
  return nil
}

func CreateProcessWithLogonW(
    username string,
    domain *string,
    password string,
    logonFlags uint32,
    applicationName *string,
    commandLine *string,
    creationFlags uint32,
    environment *Environment,
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

  r1, _, e1 := procCreateProcessWithLogonW.Call(
    uintptr(unsafe.Pointer(pUsername)),
    uintptr(unsafe.Pointer(pDomain)),
    uintptr(unsafe.Pointer(pPassword)),
    uintptr(logonFlags),
    uintptr(unsafe.Pointer(pApplicationName)),
    uintptr(unsafe.Pointer(pCommandLine)),
    uintptr(creationFlags),
    uintptr(0), // env
    uintptr(unsafe.Pointer(pCurrentDirectory)),
    uintptr(0), // si
    uintptr(unsafe.Pointer(pi)))

  if int(r1) == 0 {
    return nil, e1
  }

  return nil, nil
}
    
    
    
    
