package sub32

import (
  "syscall"
//  "unsafe"
)

var (
  advapi32 = syscall.NewLazyDLL("advapi32.dll")

  procCreateProcessWithLogonW = advapi32.NewProc("CreateProcessWithLogonW")
)

type Environment interface {}
type StartupInfo interface {}
type ProcessInformation interface {}

func CreateProcessWithLogonW(
    username string,
    domain *string,
    password string,
    logonFlags uint32,
    applicationName *string,
    commandLine *string,
    creationFlags *uint32,
    environment *Environment,
    currentDirectory *string,
    startupInfo *StartupInfo) (pi *ProcessInformation, err error) {

  return nil, nil
}
    
    
    
    
