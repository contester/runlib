package sub32

import (
  "fmt"
  "strings"
  "syscall"
  "unicode/utf16"
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
)

type Environment interface {
  ToBlock() []uint16
}

type StartupInfo interface {}
type ProcessInformation interface {}

func nilOrString(src *string) (result *uint16) {
  if src != nil {
    return syscall.StringToUTF16Ptr(*src)
  }
  return nil
}

type EnvironmentMap map[string] string

func (env EnvironmentMap) ToBlock() (result []uint16) {
  size := 1
  for k, v := range env {
    size += len(syscall.StringToUTF16(k + "=" + v))
  }

  fmt.Printf("size: %d\n", size)

  result = make([]uint16, size)

  tail := 0

  for k, v := range env {
    uline := syscall.StringToUTF16(k + "=" + v)
    copy(result[tail:], uline)
    tail += len(uline)
  }

  fmt.Printf("tail: %d\n", tail)
  result[tail] = 0

  return
}

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

func testBlock(s *uint16) {
	r := make([]string, 0, 50) // Empty with room to grow.
	for from, i, p := 0, 0, (*[1 << 24]uint16)(unsafe.Pointer(s)); true; i++ {
		if p[i] == 0 {
			// empty string marks the end
			if i <= from {
				break
			}
			r = append(r, string(utf16.Decode(p[from:i])))
			fmt.Printf("t: %s\n", string(utf16.Decode(p[from:i])))
			from = i + 1
		}
	}
}

func getEnvBlock(env Environment) *uint16 {
  if (env == nil) {
    return nil
  }

  return &env.ToBlock()[0]
}

func CreateProcessWithLogonW(
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
    
func CreateProcessW(
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
    
