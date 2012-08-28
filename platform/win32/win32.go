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
	user32   = syscall.NewLazyDLL("user32.dll")

	procCreateProcessWithLogonW   = advapi32.NewProc("CreateProcessWithLogonW")
	procCreateProcessAsUserW      = advapi32.NewProc("CreateProcessAsUserW")
	procResumeThread              = kernel32.NewProc("ResumeThread")
	procGetProcessMemoryInfo      = psapi.NewProc("GetProcessMemoryInfo")
	procLogonUserW                = advapi32.NewProc("LogonUserW")
	procLoadUserProfileW          = userenv.NewProc("LoadUserProfileW")
	procUnloadUserProfile         = userenv.NewProc("UnloadUserProfile")
	procGetProcessWindowStation   = user32.NewProc("GetProcessWindowStation")
	procGetCurrentThreadId        = kernel32.NewProc("GetCurrentThreadId")
	procGetThreadDesktop          = user32.NewProc("GetThreadDesktop")
	procCreateWindowStationW      = user32.NewProc("CreateWindowStationW")
	procSetProcessWindowStation   = user32.NewProc("SetProcessWindowStation")
	procCreateDesktopW            = user32.NewProc("CreateDesktopW")
	procSetThreadDesktop          = user32.NewProc("SetThreadDesktop")
	procGetUserObjectInformationW = user32.NewProc("GetUserObjectInformationW")
	procCloseWindowStation        = user32.NewProc("CloseWindowStation")
	procCreateJobObjectW          = kernel32.NewProc("CreateJobObjectW")
	procQueryInformationJobObject = kernel32.NewProc("QueryInformationJobObject")
	procAssignProcessToJobObject  = kernel32.NewProc("AssignProcessToJobObject")
	procVirtualAllocEx = kernel32.NewProc("VirtualAllocEx")
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

	MAXIMUM_ALLOWED = 0x2000000
	PI_NOUI         = 2
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
	Size        uint32
	Flags       uint32
	Username    *uint16
	ProfilePath *uint16
	DefaultPath *uint16
	ServerName  *uint16
	PolicyPath  *uint16
	Profile     syscall.Handle
}

type Hwinsta uintptr
type Hdesk uintptr

func MakeInheritSa() *syscall.SecurityAttributes {
	var sa syscall.SecurityAttributes
	sa.Length = uint32(unsafe.Sizeof(sa))
	sa.InheritHandle = 1
	return &sa
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

func CreateProcessAsUser(
	token syscall.Handle,
	applicationName *uint16,
	commandLine *uint16,
	procSecurity *syscall.SecurityAttributes,
	threadSecurity *syscall.SecurityAttributes,
	inheritHandles bool,
	creationFlags uint32,
	environment *uint16,
	currentDirectory *uint16,
	startupInfo *syscall.StartupInfo,
	processInformation *syscall.ProcessInformation) (err error) {

	var _p0 uint32
	if inheritHandles {
		_p0 = 1
	} else {
		_p0 = 0
	}
	r1, _, e1 := procCreateProcessAsUserW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(applicationName)),
		uintptr(unsafe.Pointer(commandLine)),
		uintptr(unsafe.Pointer(procSecurity)),
		uintptr(unsafe.Pointer(threadSecurity)),
		uintptr(_p0),
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

func GetProcessWindowStation() (Hwinsta, error) {
	r1, _, e1 := procGetProcessWindowStation.Call()
	if int(r1) == 0 {
		return Hwinsta(r1), e1
	}
	return Hwinsta(r1), nil
}

func GetCurrentThreadId() uint32 {
	r1, _, _ := procGetCurrentThreadId.Call()
	return uint32(r1)
}

func GetThreadDesktop(threadId uint32) (Hdesk, error) {
	r1, _, e1 := procGetThreadDesktop.Call(
		uintptr(threadId))
	if int(r1) == 0 {
		return Hdesk(r1), e1
	}
	return Hdesk(r1), nil
}

func CreateWindowStation(winsta *uint16, flags, desiredAccess uint32, sa *syscall.SecurityAttributes) (Hwinsta, error) {
	r1, _, e1 := procCreateWindowStationW.Call(
		uintptr(unsafe.Pointer(winsta)),
		uintptr(flags),
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(sa)))
	if int(r1) == 0 {
		return Hwinsta(r1), e1
	}
	return Hwinsta(r1), nil
}

func SetProcessWindowStation(winsta Hwinsta) error {
	r1, _, e1 := procSetProcessWindowStation.Call(
		uintptr(winsta))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

func CreateDesktop(desktop, device *uint16, devmode uintptr, flags, desiredAccess uint32, sa *syscall.SecurityAttributes) (Hdesk, error) {
	r1, _, e1 := procCreateDesktopW.Call(
		uintptr(unsafe.Pointer(desktop)),
		uintptr(unsafe.Pointer(device)),
		devmode,
		uintptr(flags),
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(sa)))
	if int(r1) == 0 {
		return Hdesk(r1), e1
	}
	return Hdesk(r1), nil
}

func SetThreadDesktop(desk Hdesk) error {
	r1, _, e1 := procSetThreadDesktop.Call(
		uintptr(desk))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

const (
	UOI_NAME = 2
)

func GetUserObjectInformation(obj syscall.Handle, index int, info unsafe.Pointer, length uint32) (uint32, error) {
	var nLength uint32
	r1, _, e1 := procGetUserObjectInformationW.Call(
		uintptr(obj),
		uintptr(index),
		uintptr(info),
		uintptr(length),
		uintptr(unsafe.Pointer(&nLength)))
	if int(r1) == 0 {
		return nLength, e1
	}
	return 0, nil
}

func GetUserObjectName(obj syscall.Handle) (string, error) {
	namebuf := make([]uint16, 256)
	_, err := GetUserObjectInformation(obj, UOI_NAME, unsafe.Pointer(&namebuf[0]), 256*2)
	if err != nil {
		return "", err
	}
	return syscall.UTF16ToString(namebuf), nil
}

func CloseWindowStation(winsta Hwinsta) error {
	r1, _, e1 := procCloseWindowStation.Call(
		uintptr(winsta))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

func CreateJobObject(sa *syscall.SecurityAttributes, name *uint16) (syscall.Handle, error) {
	r1, _, e1 := procCreateJobObjectW.Call(
		uintptr(unsafe.Pointer(sa)),
		uintptr(unsafe.Pointer(name)))
	if int(r1) == 0 {
		return syscall.InvalidHandle, e1
	}
	return syscall.Handle(r1), nil
}

func QueryInformationJobObject(job syscall.Handle, infoclass uint32, info unsafe.Pointer, length uint32) (uint32, error) {
	var nLength uint32
	r1, _, e1 := procQueryInformationJobObject.Call(
		uintptr(job),
		uintptr(infoclass),
		uintptr(info),
		uintptr(length),
		uintptr(unsafe.Pointer(&nLength)))

	if int(r1) == 0 {
		return nLength, e1
	}
	return nLength, nil
}

type JobObjectBasicAccountingInformation struct {
	TotalUserTime             uint64
	TotalKernelTime           uint64
	ThisPeriodTotalUserTime   uint64
	ThisPeriodTotalKernelTime uint64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

func GetJobObjectBasicAccountingInformation(job syscall.Handle) (*JobObjectBasicAccountingInformation, error) {
	var jinfo JobObjectBasicAccountingInformation
	_, err := QueryInformationJobObject(job, 1, unsafe.Pointer(&jinfo), uint32(unsafe.Sizeof(jinfo)))
	if err != nil {
		return nil, err
	}
	return &jinfo, nil
}

type JobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit uint64
	PerJobUserTimeLimit     uint64
	LimitFlags              uint32
	MinimumWorkingSetSize   uint32 //size_t
	MaximumWorkingSetSize   uint32 //size_t
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type IoCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type JobObjectExtendedLimitInformation struct {
	BasicLimitInformation JobObjectBasicLimitInformation
	align1                uint32
	IoInfo                IoCounters
	ProcessMemoryLimit    uint32 // size_t
	JobMemoryLimit        uint32 //
	PeakProcessMemoryUsed uint32
	PeakJobMemoryUsed     uint32
}

func GetJobObjectExtendedLimitInformation(job syscall.Handle) (*JobObjectExtendedLimitInformation, error) {
	var jinfo JobObjectExtendedLimitInformation
	_, err := QueryInformationJobObject(job, 9, unsafe.Pointer(&jinfo), uint32(unsafe.Sizeof(jinfo)))
	if err != nil {
		return nil, err
	}
	return &jinfo, nil
}

func AssignProcessToJobObject(job syscall.Handle, process syscall.Handle) error {
	r1, _, e1 := procAssignProcessToJobObject.Call(
		uintptr(job),
		uintptr(process))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

const (
	MEM_COMMIT = 0x00001000
	PAGE_READWRITE = 0x04
)

func VirtualAllocEx(process syscall.Handle, addr uintptr, size, allocType, protect uint32) (uintptr, error) {
	r1, _, e1 := procVirtualAllocEx.Call(
		uintptr(process),
		addr,
		uintptr(size),
		uintptr(allocType),
		uintptr(protect))

	if int(r1) == 0 {
		return r1, e1
	}
	return r1, nil
}

