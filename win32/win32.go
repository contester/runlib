package win32

// +build windows,386

import (
	"syscall"
	"unsafe"
)

var (
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	psapi    = syscall.NewLazyDLL("psapi.dll")
	userenv  = syscall.NewLazyDLL("userenv.dll")

	procCreateProcessWithLogonW = advapi32.NewProc("CreateProcessWithLogonW")
	procCreateProcessW          = kernel32.NewProc("CreateProcessW")
	procResumeThread            = kernel32.NewProc("ResumeThread")
	procGetProcessMemoryInfo    = psapi.NewProc("GetProcessMemoryInfo")
	procLogonUserW              = advapi32.NewProc("LogonUserW")
	procLoadUserProfileW        = userenv.NewProc("LoadUserProfileW")
	procUnloadUserProfile       = userenv.NewProc("UnloadUserProfile")
)

const (
	CREATE_BREAKAWAY_FROM_JOB = 0x01000000
	CREATE_NEW_CONSOLE        = 0x00000010
	CREATE_NEW_PROCESS_GROUP  = 0x00000200
	CREATE_SUSPENDED          = 0x00000004

	LOGON_WITH_PROFILE = 0x00000001

	STARTF_FORCEOFFFEEDBACK = 0x00000080

	FILE_FLAG_SEQUENTIAL_SCAN = 0x08000000

	LOGON32_PROVIDER_DEFAULT = 0
	LOGON32_PROVIDER_WINNT35 = 1
	LOGON32_PROVIDER_WINNT40 = 2
	LOGON32_PROVIDER_WINNT50 = 3

	LOGON32_LOGON_INTERACTIVE       = 2
	LOGON32_LOGON_NETWORK           = 3
	LOGON32_LOGON_BATCH             = 4
	LOGON32_LOGON_SERVICE           = 5
	LOGON32_LOGON_UNLOCK            = 7
	LOGON32_LOGON_NETWORK_CLEARTEXT = 8
	LOGON32_LOGON_NEW_CREDENTIALS   = 9
)

type ProcessMemoryCountersEx struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uint32
	WorkingSetSize             uint32
	QuotaPeakPagedPoolUsage    uint32
	QuotaPagedPoolUsage        uint32
	QuotaPeakNonPagedPoolUsage uint32
	QuotaNonPagedPoolUsage     uint32
	PagefileUsage              uint32
	PeakPagefileUsage          uint32
	PrivateUsage               uint32
}

type ProfileInfo struct {
	Size         uint32
	Flags        uint32
	UserName     *uint16
	ProfilePath  *uint16
	DefaultPath  *uint16
	lpServerName *uint16
	lpPolicyPath *uint16
	hProfile     syscall.Handle
}

func StringPtrToUTF16Ptr(src *string) (result *uint16) {
	if src != nil {
		return syscall.StringToUTF16Ptr(*src)
	}
	return nil
}

func ListToEnvironmentBlock(list *[]string) *uint16 {
	if list == nil {
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

func ResumeThread(thread syscall.Handle) (suspendCount int, err error) {
	r1, _, e1 := procResumeThread.Call(uintptr(thread))
	if int(r1) == -1 {
		return -1, e1
	}
	return int(r1), nil
}

func GetProcessMemoryInfo(process syscall.Handle) (pmc *ProcessMemoryCountersEx, err error) {
	pmc = &ProcessMemoryCountersEx{}
	pmc.Cb = uint32(unsafe.Sizeof(*pmc))
	r1, _, e1 := procGetProcessMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(pmc)), uintptr(pmc.Cb))
	if int(r1) == 0 {
		return nil, e1
	}
	return pmc, nil
}

func LogonUser(username *uint16, domain *uint16, password *uint16, logonType uint32, logonProvider uint32) (token syscall.Handle, err error) {
	r1, _, e1 := procLogonUserW.Call(
		uintptr(unsafe.Pointer(username)),
		uintptr(unsafe.Pointer(domain)),
		uintptr(unsafe.Pointer(password)),
		uintptr(logonType),
		uintptr(logonProvider),
		uintptr(unsafe.Pointer(&token)))

	if int(r1) == 0 {
		return syscall.InvalidHandle, e1
	}
	return
}

func LoadUserProfile(token syscall.Handle, pinfo *ProfileInfo) error {
	r1, _, e1 := procLoadUserProfileW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(pinfo)))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

func UnloadUserProfile(token, profile syscall.Handle) error {
	r1, _, e1 := procUnloadUserProfile.Call(
		uintptr(token),
		uintptr(profile))
	if int(r1) == 0 {
		return e1
	}
	return nil
}
