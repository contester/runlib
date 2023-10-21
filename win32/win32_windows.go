package win32

import (
	"fmt"
	"os"
	"runtime"
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
	procGetThreadDesktop          = user32.NewProc("GetThreadDesktop")
	procCreateWindowStationW      = user32.NewProc("CreateWindowStationW")
	procSetProcessWindowStation   = user32.NewProc("SetProcessWindowStation")
	procCreateDesktopW            = user32.NewProc("CreateDesktopW")
	procSetThreadDesktop          = user32.NewProc("SetThreadDesktop")
	procGetUserObjectInformationW = user32.NewProc("GetUserObjectInformationW")
	procCloseWindowStation        = user32.NewProc("CloseWindowStation")
	procCreateJobObjectW          = kernel32.NewProc("CreateJobObjectW")
	procQueryInformationJobObject = kernel32.NewProc("QueryInformationJobObject")
	procSetInformationJobObject   = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject  = kernel32.NewProc("AssignProcessToJobObject")
	procVirtualAllocEx            = kernel32.NewProc("VirtualAllocEx")
	procWriteProcessMemory        = kernel32.NewProc("WriteProcessMemory")
	procGetModuleHandleW          = kernel32.NewProc("GetModuleHandleW")
	procCreateRemoteThread        = kernel32.NewProc("CreateRemoteThread")
	procVirtualFreeEx             = kernel32.NewProc("VirtualFreeEx")
	procSetProcessAffinityMask    = kernel32.NewProc("SetProcessAffinityMask")
	procGetProcessAffinityMask    = kernel32.NewProc("GetProcessAffinityMask")
	procVerifyVersionInfoW        = kernel32.NewProc("VerifyVersionInfoW")
	procVerSetConditionMask       = kernel32.NewProc("VerSetConditionMask")
	procGetBinaryTypeW            = kernel32.NewProc("GetBinaryTypeW")
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
	PI_NOUI         = 1

	VER_BUILDNUMBER      = 0x0000004
	VER_MAJORVERSION     = 0x0000002
	VER_MINORVERSION     = 0x0000001
	VER_PLATFORMID       = 0x0000008
	VER_PRODUCT_TYPE     = 0x0000080
	VER_SERVICEPACKMAJOR = 0x0000020
	VER_SERVICEPACKMINOR = 0x0000010
	VER_SUITENAME        = 0x0000040

	VER_EQUAL         = 1
	VER_GREATER       = 2
	VER_GREATER_EQUAL = 3
	VER_LESS          = 4
	VER_LESS_EQUAL    = 5

	ERROR_OLD_WIN_VERSION syscall.Errno = 1150
)

type OSVersionInfoEx struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformId        uint32
	CSDVersion        [128]uint16
	ServicePackMajor  uint16
	ServicePackMinor  uint16
	SuiteMask         uint16
	ProductType       byte
	Reserve           byte
}

func IsWindows8OrGreater() bool {
	cm := VerSetConditionMask(0, VER_MAJORVERSION, VER_GREATER_EQUAL)
	cm = VerSetConditionMask(cm, VER_MINORVERSION, VER_GREATER_EQUAL)
	cm = VerSetConditionMask(cm, VER_SERVICEPACKMAJOR, VER_GREATER_EQUAL)
	cm = VerSetConditionMask(cm, VER_SERVICEPACKMINOR, VER_GREATER_EQUAL)
	r, _ := VerifyWindowsInfoW(OSVersionInfoEx{
		MajorVersion: 6,
		MinorVersion: 2,
	}, VER_MAJORVERSION|VER_MINORVERSION|VER_SERVICEPACKMAJOR|VER_SERVICEPACKMINOR, cm)
	return r
}

type ProcessMemoryCountersEx struct {
	Cb                         uint32  // DWORD
	PageFaultCount             uint32  // DWORD
	PeakWorkingSetSize         uintptr // SIZE_T
	WorkingSetSize             uintptr // SIZE_T
	QuotaPeakPagedPoolUsage    uintptr // SIZE_T
	QuotaPagedPoolUsage        uintptr // SIZE_T
	QuotaPeakNonPagedPoolUsage uintptr // SIZE_T
	QuotaNonPagedPoolUsage     uintptr // SIZE_T
	PagefileUsage              uintptr // SIZE_T
	PeakPagefileUsage          uintptr // SIZE_T
	PrivateUsage               uintptr // SIZE_T
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

func UTF16PtrFromStringOrNil(src string) (*uint16, error) {
	if src == "" {
		return nil, nil
	}
	return syscall.UTF16PtrFromString(src)
}

func ListToEnvironmentBlock(list []string) (*uint16, error) {
	result := make([]uint16, 0, 16*1024)
	for _, v := range list {
		c, err := syscall.UTF16FromString(v)
		if err != nil {
			return nil, err
		}
		result = append(result, c...)
	}
	result = append(result, 0)
	return &result[0], nil
}

type ProcessEnvironmentOptions struct {
	NoInherit bool
	Env       []string
}

func CreateProcessWithLogonW(
	username,
	domain,
	password string,
	logonFlags uint32,
	applicationName,
	commandLine string,
	creationFlags uint32,
	environment ProcessEnvironmentOptions,
	currentDirectory string,
	startupInfo *syscall.StartupInfo,
	processInformation *syscall.ProcessInformation) error {
	pUsername, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	pDomain, err := syscall.UTF16PtrFromString(domain)
	if err != nil {
		return err
	}
	pPassword, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return err
	}
	pApplicationName, err := UTF16PtrFromStringOrNil(applicationName)
	if err != nil {
		return err
	}
	pCommandLine, err := UTF16PtrFromStringOrNil(commandLine)
	if err != nil {
		return err
	}
	pCurrentDirectory, err := UTF16PtrFromStringOrNil(currentDirectory)
	if err != nil {
		return err
	}
	var pEnvironment *uint16
	if environment.NoInherit {
		pEnvironment, err = ListToEnvironmentBlock(environment.Env)
		if err != nil {
			return err
		}
	}

	r1, _, e1 := procCreateProcessWithLogonW.Call(
		uintptr(unsafe.Pointer(pUsername)),
		uintptr(unsafe.Pointer(pDomain)),
		uintptr(unsafe.Pointer(pPassword)),
		uintptr(logonFlags),
		uintptr(unsafe.Pointer(pApplicationName)),
		uintptr(unsafe.Pointer(pCommandLine)),
		uintptr(creationFlags),
		uintptr(unsafe.Pointer(pEnvironment)),
		uintptr(unsafe.Pointer(pCurrentDirectory)),
		uintptr(unsafe.Pointer(startupInfo)),
		uintptr(unsafe.Pointer(processInformation)))
	runtime.KeepAlive(pUsername)
	runtime.KeepAlive(pDomain)
	runtime.KeepAlive(pPassword)
	runtime.KeepAlive(pApplicationName)
	runtime.KeepAlive(pCommandLine)
	runtime.KeepAlive(pEnvironment)
	runtime.KeepAlive(pCurrentDirectory)
	runtime.KeepAlive(startupInfo)
	runtime.KeepAlive(processInformation)
	if int(r1) == 0 {
		return os.NewSyscallError("CreateProcessWithLogonW", e1)
	}
	return nil
}

func boolToUint32(src bool) uint32 {
	if src {
		return 1
	}
	return 0
}

func CreateProcessAsUser(
	token syscall.Handle,
	applicationName,
	commandLine string,
	procSecurity *syscall.SecurityAttributes,
	threadSecurity *syscall.SecurityAttributes,
	inheritHandles bool,
	creationFlags uint32,
	environment ProcessEnvironmentOptions,
	currentDirectory string,
	startupInfo *syscall.StartupInfo,
	processInformation *syscall.ProcessInformation) error {

	pApplicationName, err := UTF16PtrFromStringOrNil(applicationName)
	if err != nil {
		return err
	}
	pCommandLine, err := UTF16PtrFromStringOrNil(commandLine)
	if err != nil {
		return err
	}
	pCurrentDirectory, err := UTF16PtrFromStringOrNil(currentDirectory)
	if err != nil {
		return err
	}
	var pEnvironment *uint16
	if environment.NoInherit {
		pEnvironment, err = ListToEnvironmentBlock(environment.Env)
		if err != nil {
			return err
		}
	}

	r1, _, e1 := procCreateProcessAsUserW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(pApplicationName)),
		uintptr(unsafe.Pointer(pCommandLine)),
		uintptr(unsafe.Pointer(procSecurity)),
		uintptr(unsafe.Pointer(threadSecurity)),
		uintptr(boolToUint32(inheritHandles)),
		uintptr(creationFlags),
		uintptr(unsafe.Pointer(pEnvironment)), // env
		uintptr(unsafe.Pointer(pCurrentDirectory)),
		uintptr(unsafe.Pointer(startupInfo)),
		uintptr(unsafe.Pointer(processInformation)))
	runtime.KeepAlive(pApplicationName)
	runtime.KeepAlive(pCommandLine)
	runtime.KeepAlive(procSecurity)
	runtime.KeepAlive(threadSecurity)
	runtime.KeepAlive(pEnvironment)
	runtime.KeepAlive(pCurrentDirectory)
	runtime.KeepAlive(startupInfo)
	runtime.KeepAlive(processInformation)
	if int(r1) == 0 {
		return os.NewSyscallError("CreateProcessAsUser", e1)
	}
	return nil
}

func CreateProcess(
	applicationName,
	commandLine string,
	procSecurity *syscall.SecurityAttributes,
	threadSecurity *syscall.SecurityAttributes,
	inheritHandles bool,
	creationFlags uint32,
	environment ProcessEnvironmentOptions,
	currentDirectory string,
	startupInfo *syscall.StartupInfo,
	processInformation *syscall.ProcessInformation) error {

	pApplicationName, err := UTF16PtrFromStringOrNil(applicationName)
	if err != nil {
		return err
	}
	pCommandLine, err := UTF16PtrFromStringOrNil(commandLine)
	if err != nil {
		return err
	}
	pCurrentDirectory, err := UTF16PtrFromStringOrNil(currentDirectory)
	if err != nil {
		return err
	}
	var pEnvironment *uint16
	if environment.NoInherit {
		pEnvironment, err = ListToEnvironmentBlock(environment.Env)
		if err != nil {
			return err
		}
	}

	err = syscall.CreateProcess(
		pApplicationName,
		pCommandLine,
		procSecurity,
		threadSecurity,
		inheritHandles,
		creationFlags,
		pEnvironment,
		pCurrentDirectory,
		startupInfo,
		processInformation)
	runtime.KeepAlive(pApplicationName)
	runtime.KeepAlive(pCommandLine)
	runtime.KeepAlive(procSecurity)
	runtime.KeepAlive(threadSecurity)
	runtime.KeepAlive(pEnvironment)
	runtime.KeepAlive(pCurrentDirectory)
	runtime.KeepAlive(startupInfo)
	runtime.KeepAlive(processInformation)
	return os.NewSyscallError("CreateProcess", err)
}

func ResumeThread(thread syscall.Handle) (suspendCount int, err error) {
	r1, _, e1 := procResumeThread.Call(uintptr(thread))
	if int(r1) == -1 {
		return -1, os.NewSyscallError("ResumeThread", e1)
	}
	return int(r1), nil
}

func GetProcessMemoryInfo(process syscall.Handle) (pmc *ProcessMemoryCountersEx, err error) {
	pmc = &ProcessMemoryCountersEx{
		Cb: uint32(unsafe.Sizeof(*pmc)),
	}
	if r1, _, e1 := procGetProcessMemoryInfo.Call(uintptr(process), uintptr(unsafe.Pointer(pmc)),
		unsafe.Sizeof(*pmc)); int(r1) == 0 {
		return nil, os.NewSyscallError("GetProcessMemoryInfo", e1)
	}
	return pmc, nil
}

func LogonUser(username, domain, password string, logonType uint32, logonProvider uint32) (token syscall.Handle, err error) {
	pUsername, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	pDomain, err := syscall.UTF16PtrFromString(domain)
	if err != nil {
		return syscall.InvalidHandle, err
	}
	pPassword, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return syscall.InvalidHandle, err
	}

	r1, _, e1 := procLogonUserW.Call(
		uintptr(unsafe.Pointer(pUsername)),
		uintptr(unsafe.Pointer(pDomain)),
		uintptr(unsafe.Pointer(pPassword)),
		uintptr(logonType),
		uintptr(logonProvider),
		uintptr(unsafe.Pointer(&token)))
	runtime.KeepAlive(pUsername)
	runtime.KeepAlive(pDomain)
	runtime.KeepAlive(pPassword)
	if int(r1) == 0 {
		if domain == "" {
			domain = "."
		}
		combinedCredential := domain + "\\" + username
		return syscall.InvalidHandle, fmt.Errorf("win32.LogonUser(%q): %w", combinedCredential, e1)
	}
	return token, nil
}

func LoadUserProfile(token syscall.Handle, pinfo *ProfileInfo) error {
	r1, _, e1 := procLoadUserProfileW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(pinfo)))
	runtime.KeepAlive(pinfo)
	if int(r1) == 0 {
		return os.NewSyscallError("LoadUserProfile", e1)
	}
	return nil
}

func UnloadUserProfile(token, profile syscall.Handle) error {
	if r1, _, e1 := procUnloadUserProfile.Call(
		uintptr(token),
		uintptr(profile)); int(r1) == 0 {
		return os.NewSyscallError("UnloadUserProfile", e1)
	}
	return nil
}

func GetProcessWindowStation() (Hwinsta, error) {
	r1, _, e1 := procGetProcessWindowStation.Call()
	if int(r1) == 0 {
		return Hwinsta(r1), os.NewSyscallError("GetProcessWindowStation", e1)
	}
	return Hwinsta(r1), nil
}

func GetThreadDesktop(threadId uint32) (Hdesk, error) {
	r1, _, e1 := procGetThreadDesktop.Call(
		uintptr(threadId))
	if int(r1) == 0 {
		return Hdesk(r1), os.NewSyscallError("GetThreadDesktop", e1)
	}
	return Hdesk(r1), nil
}

func CreateWindowStation(winsta *uint16, flags, desiredAccess uint32, sa *syscall.SecurityAttributes) (Hwinsta, error) {
	r1, _, e1 := procCreateWindowStationW.Call(
		uintptr(unsafe.Pointer(winsta)),
		uintptr(flags),
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(sa)))
	runtime.KeepAlive(winsta)
	runtime.KeepAlive(sa)
	if int(r1) == 0 {
		return Hwinsta(r1), os.NewSyscallError("CreateWindowStation", e1)
	}
	return Hwinsta(r1), nil
}

func SetProcessWindowStation(winsta Hwinsta) error {
	r1, _, e1 := procSetProcessWindowStation.Call(
		uintptr(winsta))
	if int(r1) == 0 {
		return os.NewSyscallError("SetProcessWindowStation", e1)
	}
	return nil
}

func CreateDesktop(desktop string, flags, desiredAccess uint32, sa *syscall.SecurityAttributes) (Hdesk, error) {
	pDesktop, err := syscall.UTF16PtrFromString(desktop)
	if err != nil {
		return 0, err
	}
	r1, _, e1 := procCreateDesktopW.Call(
		uintptr(unsafe.Pointer(pDesktop)),
		0,
		0,
		uintptr(flags),
		uintptr(desiredAccess),
		uintptr(unsafe.Pointer(sa)))
	runtime.KeepAlive(pDesktop)
	runtime.KeepAlive(sa)
	if int(r1) == 0 {
		return Hdesk(r1), os.NewSyscallError("CreateDesktop", e1)
	}
	return Hdesk(r1), nil
}

func SetThreadDesktop(desk Hdesk) error {
	r1, _, e1 := procSetThreadDesktop.Call(
		uintptr(desk))
	if int(r1) == 0 {
		return os.NewSyscallError("SetThreadDesktop", e1)
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
		return nLength, os.NewSyscallError("GetUserObjectInformation", e1)
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
		return os.NewSyscallError("CloseWindowStation", e1)
	}
	return nil
}

func CreateJobObject(sa *syscall.SecurityAttributes, name *uint16) (syscall.Handle, error) {
	r1, _, e1 := procCreateJobObjectW.Call(
		uintptr(unsafe.Pointer(sa)),
		uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(sa)
	runtime.KeepAlive(name)
	if int(r1) == 0 {
		return syscall.InvalidHandle, os.NewSyscallError("CreateJobObject", e1)
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
		return nLength, os.NewSyscallError("QueryInformationJobObject", e1)
	}
	return nLength, nil
}

func SetInformationJobObject(job syscall.Handle, infoclass uint32, info unsafe.Pointer, length uint32) error {
	r1, _, e1 := procSetInformationJobObject.Call(
		uintptr(job),
		uintptr(infoclass),
		uintptr(info),
		uintptr(length))
	if int(r1) == 0 {
		return os.NewSyscallError("SetInformationJobObject", e1)
	}
	return nil
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

type JobObjectBasicUiRestrictions struct {
	UIRestrictionClass uint32
}

const (
	JOB_OBJECT_UILIMIT_DESKTOP          = 0x40
	JOB_OBJECT_UILIMIT_DISPLAYSETTINGS  = 0x10
	JOB_OBJECT_UILIMIT_EXITWINDOWS      = 0x80
	JOB_OBJECT_UILIMIT_GLOBALATOMS      = 0x20
	JOB_OBJECT_UILIMIT_HANDLES          = 1
	JOB_OBJECT_UILIMIT_READCLIPBOARD    = 2
	JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS = 8
	JOB_OBJECT_UILIMIT_WRITECLIPBOARD   = 4
)

func GetJobObjectBasicAccountingInformation(job syscall.Handle) (*JobObjectBasicAccountingInformation, error) {
	var jinfo JobObjectBasicAccountingInformation
	_, err := QueryInformationJobObject(job, 1, unsafe.Pointer(&jinfo), uint32(unsafe.Sizeof(jinfo)))
	if err != nil {
		return nil, err
	}
	return &jinfo, nil
}

type JobObjectBasicLimitInformation struct {
	PerProcessUserTimeLimit uint64  // LARGE_INTEGER
	PerJobUserTimeLimit     uint64  // LARGE_INTEGER
	LimitFlags              uint32  // DWORD
	MinimumWorkingSetSize   uintptr // SIZE_T
	MaximumWorkingSetSize   uintptr // SIZE_T
	ActiveProcessLimit      uint32  // DWORD
	Affinity                uintptr // ULONG_PTR
	PriorityClass           uint32  // DWORD
	SchedulingClass         uint32  // DWORD
}

const (
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE          = 0x2000
	JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION = 0x400
	JOB_OBJECT_LIMIT_ACTIVE_PROCESS             = 8
	JOB_OBJECT_LIMIT_JOB_MEMORY                 = 0x200
	JOB_OBJECT_LIMIT_JOB_TIME                   = 4
	JOB_OBJECT_LIMIT_PROCESS_MEMORY             = 0x100
	JOB_OBJECT_LIMIT_PROCESS_TIME               = 2
	JOB_OBJECT_LIMIT_WORKINGSET                 = 1
	JOB_OBJECT_LIMIT_AFFINITY                   = 0x00000010
)

type IoCounters struct {
	ReadOperationCount  uint64 // ULONGLONG
	WriteOperationCount uint64 // ULONGLONG
	OtherOperationCount uint64 // ULONGLONG
	ReadTransferCount   uint64 // ULONGLONG
	WriteTransferCount  uint64 // ULONGLONG
	OtherTransferCount  uint64 // ULONGLONG
}

func GetJobObjectExtendedLimitInformation(job syscall.Handle) (*JobObjectExtendedLimitInformation, error) {
	var jinfo JobObjectExtendedLimitInformation
	_, err := QueryInformationJobObject(job, 9, unsafe.Pointer(&jinfo), uint32(unsafe.Sizeof(jinfo)))
	if err != nil {
		return nil, err
	}
	return &jinfo, nil
}

func SetJobObjectBasicUiRestrictions(job syscall.Handle, info *JobObjectBasicUiRestrictions) error {
	err := SetInformationJobObject(job, 4, unsafe.Pointer(info), uint32(unsafe.Sizeof(*info)))
	runtime.KeepAlive(info)
	return err
}

func SetJobObjectExtendedLimitInformation(job syscall.Handle, info *JobObjectExtendedLimitInformation) error {
	err := SetInformationJobObject(job, 9, unsafe.Pointer(info), uint32(unsafe.Sizeof(*info)))
	runtime.KeepAlive(info)
	return err
}

func AssignProcessToJobObject(job syscall.Handle, process syscall.Handle) error {
	r1, _, e1 := procAssignProcessToJobObject.Call(
		uintptr(job),
		uintptr(process))
	if int(r1) == 0 {
		return os.NewSyscallError("AssignProcessToJobObject", e1)
	}
	return nil
}

const (
	MEM_COMMIT     = 0x00001000
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
		return r1, os.NewSyscallError("VirtualAllocEx", e1)
	}
	return r1, nil
}

func WriteProcessMemory(process syscall.Handle, addr uintptr, buf unsafe.Pointer, size uint32) (uint32, error) {
	var nLength uint32
	r1, _, e1 := procWriteProcessMemory.Call(
		uintptr(process),
		addr,
		uintptr(buf),
		uintptr(size),
		uintptr(unsafe.Pointer(&nLength)))

	if int(r1) == 0 {
		return nLength, os.NewSyscallError("WriteProcessMemory", e1)
	}
	return nLength, nil
}

func GetModuleHandle(name string) (syscall.Handle, error) {
	pName, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	r1, _, e1 := procGetModuleHandleW.Call(uintptr(unsafe.Pointer(pName)))
	runtime.KeepAlive(pName)
	if int(r1) == 0 {
		return syscall.InvalidHandle, os.NewSyscallError("GetModuleHandle", e1)
	}
	return syscall.Handle(r1), nil
}

func CreateRemoteThread(process syscall.Handle, sa *syscall.SecurityAttributes, stackSize uint32, startAddress,
	parameter uintptr, creationFlags uint32) (syscall.Handle, uint32, error) {
	var threadId uint32
	r1, _, e1 := procCreateRemoteThread.Call(
		uintptr(process),
		uintptr(unsafe.Pointer(sa)),
		uintptr(stackSize),
		startAddress,
		parameter,
		uintptr(creationFlags),
		uintptr(unsafe.Pointer(&threadId)))
	runtime.KeepAlive(sa)
	if int(r1) == 0 {
		return syscall.InvalidHandle, 0, os.NewSyscallError("CreateRemoteThread", e1)
	}
	return syscall.Handle(r1), threadId, nil
}

const (
	MEM_RELEASE = 0x8000
)

func VirtualFreeEx(process syscall.Handle, addr uintptr, size, freeType uint32) error {
	r1, _, e1 := procVirtualFreeEx.Call(
		uintptr(process),
		addr,
		uintptr(size),
		uintptr(freeType))

	if int(r1) == 0 {
		return os.NewSyscallError("VirtualFreeEx", e1)
	}
	return nil
}

func SetInheritHandle(h syscall.Handle, inherit bool) error {
	var v uint32
	if inherit {
		v = syscall.HANDLE_FLAG_INHERIT
	}
	return os.NewSyscallError("SetHandleInformation",
		syscall.SetHandleInformation(syscall.Handle(h), syscall.HANDLE_FLAG_INHERIT, v))
}

func SetProcessAffinityMask(process syscall.Handle, mask uint64) error {
	r1, _, e1 := procSetProcessAffinityMask.Call(
		uintptr(process),
		uintptr(mask))

	if int(r1) == 0 {
		return os.NewSyscallError("SetProcessAffinityMask", e1)
	}
	return nil
}

func GetProcessAffinityMask(process syscall.Handle) (processMask, systemMask uint64, err error) {
	r1, _, e1 := procGetProcessAffinityMask.Call(
		uintptr(process),
		uintptr(unsafe.Pointer(&processMask)),
		uintptr(unsafe.Pointer(&systemMask)))

	if int(r1) == 0 {
		return 0, 0, os.NewSyscallError("GetProcessAffinityMask", e1)
	}
	return processMask, systemMask, nil
}

const (
	SCS_32BIT_BINARY = 0
	SCS_64BIT_BINARY = 6
	SCS_DOS_BINARY   = 1
	SCS_OS216_BINARY = 5
	SCS_PIF_BINARY   = 3
	SCS_POSIX_BINARY = 4
	SCS_WOW_BINARY   = 2
)

func GetBinaryType(applicationName string) (uint64, error) {
	lp, err := syscall.UTF16PtrFromString(applicationName)
	if err != nil {
		return 0, err
	}

	var result uint64
	r1, _, e1 := procGetBinaryTypeW.Call(uintptr(unsafe.Pointer(lp)), uintptr(unsafe.Pointer(&result)))
	runtime.KeepAlive(lp)
	if int(r1) == 0 {
		return 0, os.NewSyscallError("GetBinaryType", e1)
	}
	return result, nil
}
