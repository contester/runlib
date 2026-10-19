package win32

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// EXTENDED_STARTUPINFO_PRESENT tells CreateProcess* that the STARTUPINFO is
// actually a STARTUPINFOEX carrying a PROC_THREAD_ATTRIBUTE_LIST.
const EXTENDED_STARTUPINFO_PRESENT = 0x00080000

// procCreateProcessW is the token-less launcher used by the handle-list path
// (the plain syscall.CreateProcess wrapper cannot pass a STARTUPINFOEX).
var procCreateProcessW = kernel32.NewProc("CreateProcessW")

// startupInfoEx mirrors windows.StartupInfoEx but embeds syscall.StartupInfo
// (the type the redirect setup fills), so a *startupInfoEx can be handed to
// the CreateProcess* thunks as a *syscall.StartupInfo.
type startupInfoEx struct {
	syscall.StartupInfo
	attributeList *windows.ProcThreadAttributeList
}

// launchArgs is the resolved STARTUPINFO pointer, creation flags and
// bInheritHandles value for one launch, plus whatever must be kept alive and
// freed until after the CreateProcess call returns.
type launchArgs struct {
	siPtr   unsafe.Pointer
	flags   uint32
	inherit uintptr
	al      *windows.ProcThreadAttributeListContainer // nil when no list
	keep    interface{}                               // *startupInfoEx, held alive
}

// prepareLaunch builds the launch arguments. With a non-empty inheritList it
// produces a STARTUPINFOEX whose PROC_THREAD_ATTRIBUTE_HANDLE_LIST names
// exactly those handles, so the child inherits only them even though
// bInheritHandles is TRUE — making inheritance race-free without ForkLock.
// With an empty list the child inherits nothing (bInheritHandles=FALSE), which
// is likewise race-free: a child that inherits nothing cannot pick up another
// launch's momentarily-inheritable handles.
func prepareLaunch(startupInfo *syscall.StartupInfo, inheritList []syscall.Handle, flags uint32) (*launchArgs, error) {
	if len(inheritList) == 0 {
		return &launchArgs{
			siPtr:   unsafe.Pointer(startupInfo),
			flags:   flags,
			inherit: 0,
		}, nil
	}
	al, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	if err := al.Update(windows.PROC_THREAD_ATTRIBUTE_HANDLE_LIST,
		unsafe.Pointer(&inheritList[0]),
		uintptr(len(inheritList))*unsafe.Sizeof(inheritList[0])); err != nil {
		al.Delete()
		return nil, err
	}
	siEx := &startupInfoEx{}
	siEx.StartupInfo = *startupInfo
	siEx.attributeList = al.List()
	siEx.StartupInfo.Cb = uint32(unsafe.Sizeof(*siEx))
	return &launchArgs{
		siPtr:   unsafe.Pointer(siEx),
		flags:   flags | EXTENDED_STARTUPINFO_PRESENT,
		inherit: 1,
		al:      al,
		keep:    siEx,
	}, nil
}

func (la *launchArgs) done() {
	if la.al != nil {
		la.al.Delete()
	}
}

// CreateProcessAsUserInheritList is CreateProcessAsUser that inherits only the
// handles in inheritList (via STARTUPINFOEX), so it needs no ForkLock.
func CreateProcessAsUserInheritList(
	token syscall.Handle,
	applicationName, commandLine string,
	inheritList []syscall.Handle,
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

	la, err := prepareLaunch(startupInfo, inheritList, creationFlags)
	if err != nil {
		return err
	}
	defer la.done()

	r1, _, e1 := procCreateProcessAsUserW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(pApplicationName)),
		uintptr(unsafe.Pointer(pCommandLine)),
		0,
		0,
		la.inherit,
		uintptr(la.flags),
		uintptr(unsafe.Pointer(pEnvironment)),
		uintptr(unsafe.Pointer(pCurrentDirectory)),
		uintptr(la.siPtr),
		uintptr(unsafe.Pointer(processInformation)))
	runtime.KeepAlive(pApplicationName)
	runtime.KeepAlive(pCommandLine)
	runtime.KeepAlive(pEnvironment)
	runtime.KeepAlive(pCurrentDirectory)
	runtime.KeepAlive(inheritList)
	runtime.KeepAlive(la.al)
	runtime.KeepAlive(la.keep)
	runtime.KeepAlive(processInformation)
	if int(r1) == 0 {
		return os.NewSyscallError("CreateProcessAsUser", e1)
	}
	return nil
}

// CreateProcessInheritList is the token-less CreateProcess that inherits only
// the handles in inheritList (via STARTUPINFOEX), so it needs no ForkLock.
func CreateProcessInheritList(
	applicationName, commandLine string,
	inheritList []syscall.Handle,
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

	la, err := prepareLaunch(startupInfo, inheritList, creationFlags)
	if err != nil {
		return err
	}
	defer la.done()

	r1, _, e1 := procCreateProcessW.Call(
		uintptr(unsafe.Pointer(pApplicationName)),
		uintptr(unsafe.Pointer(pCommandLine)),
		0,
		0,
		la.inherit,
		uintptr(la.flags),
		uintptr(unsafe.Pointer(pEnvironment)),
		uintptr(unsafe.Pointer(pCurrentDirectory)),
		uintptr(la.siPtr),
		uintptr(unsafe.Pointer(processInformation)))
	runtime.KeepAlive(pApplicationName)
	runtime.KeepAlive(pCommandLine)
	runtime.KeepAlive(pEnvironment)
	runtime.KeepAlive(pCurrentDirectory)
	runtime.KeepAlive(inheritList)
	runtime.KeepAlive(la.al)
	runtime.KeepAlive(la.keep)
	runtime.KeepAlive(processInformation)
	if int(r1) == 0 {
		return os.NewSyscallError("CreateProcess", e1)
	}
	return nil
}
