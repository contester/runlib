package subprocess

import (
	"bytes"
	l4g "code.google.com/p/log4go"
	"fmt"
	"github.com/contester/runlib/win32"
	"github.com/contester/runlib/tools"
	"os"
	"syscall"
	"time"
	"unsafe"
)

type PlatformData struct {
	hProcess syscall.Handle
	hThread  syscall.Handle
	hJob     syscall.Handle

	hStdIn  syscall.Handle
	hStdOut syscall.Handle
	hStdErr syscall.Handle
}

type PlatformOptions struct {
	Desktop      string
	InjectDLL    string
	LoadLibraryW uintptr
}

type LoginInfo struct {
	Username, Password string
	HUser, HProfile    syscall.Handle
}

func NewLoginInfo(username, password string) (*LoginInfo, error) {
	result := &LoginInfo{Username: username, Password: password}
	err := result.Prepare()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// 1. setup; create redirects
// 2. createFrozen
// 3. setupOnFrozen; close redirects, extra memory; start reader/waiter threads; inject dll
// 4. unfreeze
// 5. wait

func (d *SubprocessData) wOutputRedirect(w *Redirect, b *bytes.Buffer) (syscall.Handle, error) {
	f, err := d.SetupOutput(w, b)
	if err != nil || f == nil {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(f.Fd()), nil
}

func (d *SubprocessData) wInputRedirect(w *Redirect) (syscall.Handle, error) {
	f, err := d.SetupInput(w)
	if err != nil || f == nil {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(f.Fd()), nil
}

func (d *SubprocessData) wAllRedirects(s *Subprocess, si *syscall.StartupInfo) error {
	var err error

	if si.StdInput, err = d.wInputRedirect(s.StdIn); err != nil {
		return err
	}
	if si.StdOutput, err = d.wOutputRedirect(s.StdOut, &d.stdOut); err != nil {
		return err
	}
	if si.StdErr, err = d.wOutputRedirect(s.StdErr, &d.stdErr); err != nil {
		return err
	}
	if si.StdInput != syscall.InvalidHandle ||
		si.StdOutput != syscall.InvalidHandle ||
		si.StdErr != syscall.InvalidHandle {
		si.Flags |= syscall.STARTF_USESTDHANDLES

		if si.StdInput == syscall.InvalidHandle {
			si.StdInput, _ = syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
		}
		if si.StdOutput == syscall.InvalidHandle {
			si.StdOutput, _ = syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
		}
		if si.StdErr == syscall.InvalidHandle {
			si.StdErr, _ = syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
		}
	}
	return nil
}

func wSetInherit(si *syscall.StartupInfo) {
	if si.StdInput != syscall.InvalidHandle {
		win32.SetInheritHandle(si.StdInput, true)
	}
	if si.StdOutput != syscall.InvalidHandle {
		win32.SetInheritHandle(si.StdOutput, true)
	}
	if si.StdErr != syscall.InvalidHandle {
		win32.SetInheritHandle(si.StdErr, true)
	}
	// TODO: errors
}

func terminateProcessLoop(process syscall.Handle) error {
	for waitResult := uint32(syscall.WAIT_TIMEOUT); waitResult == syscall.WAIT_TIMEOUT; {
		syscall.TerminateProcess(process, 0)
		waitResult, _ = syscall.WaitForSingleObject(process, 100)
	}
	return nil
}

func (d *PlatformData) terminateAndClose() (err error) {
	if err = terminateProcessLoop(d.hProcess); err != nil {
		return
	}
	syscall.CloseHandle(d.hThread)
	syscall.CloseHandle(d.hProcess)
	return
}


func (sub *Subprocess) CreateFrozen() (*SubprocessData, error) {
	d := &SubprocessData{}

	si := &syscall.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Flags = win32.STARTF_FORCEOFFFEEDBACK | syscall.STARTF_USESHOWWINDOW
	si.ShowWindow = syscall.SW_SHOWMINNOACTIVE
	if !sub.NoJob && sub.Options != nil && sub.Options.Desktop != "" {
		si.Desktop = syscall.StringToUTF16Ptr(sub.Options.Desktop)
	}

	ec := tools.NewContext("CreateFrozen")

	e := d.wAllRedirects(sub, si)
	if e != nil {
		return nil, e
	}

	pi := &syscall.ProcessInformation{}

	applicationName := win32.StringPtrToUTF16Ptr(sub.Cmd.ApplicationName)
	commandLine := win32.StringPtrToUTF16Ptr(sub.Cmd.CommandLine)
	environment := win32.ListToEnvironmentBlock(sub.Environment)
	currentDirectory := win32.StringPtrToUTF16Ptr(sub.CurrentDirectory)

	var syscallName string

	syscall.ForkLock.Lock()
	wSetInherit(si)

	if sub.Login != nil {
		if sub.NoJob {
			syscallName = "CreateProcessWithLogonW"
			e = win32.CreateProcessWithLogonW(
				syscall.StringToUTF16Ptr(sub.Login.Username),
				syscall.StringToUTF16Ptr("."),
				syscall.StringToUTF16Ptr(sub.Login.Password),
				win32.LOGON_WITH_PROFILE,
				applicationName,
				commandLine,
				win32.CREATE_SUSPENDED|syscall.CREATE_UNICODE_ENVIRONMENT,
				environment,
				currentDirectory,
				si,
				pi)
		} else {
			syscallName = "CreateProcessAsUser"
			e = win32.CreateProcessAsUser(
				sub.Login.HUser,
				applicationName,
				commandLine,
				nil,
				nil,
				true,
				win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|
					syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
				environment,
				currentDirectory,
				si,
				pi)
		}
	} else {
		syscallName = "CreateProcess"
		e = syscall.CreateProcess(
			applicationName,
			commandLine,
			nil,
			nil,
			true,
			win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|
				syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
			environment,
			currentDirectory,
			si,
			pi)
	}

	closeDescriptors(d.closeAfterStart)
	syscall.ForkLock.Unlock()

	if e != nil {
		if errno, ok := e.(syscall.Errno); ok && errno == syscall.Errno(136) {
			e = tools.NewError(e, ERR_USER)
		}
		return nil, ec.NewError(e, syscallName)
	}

	d.platformData.hProcess = pi.Process
	d.platformData.hThread = pi.Thread
	d.platformData.hJob = syscall.InvalidHandle

	e = InjectDll(sub, d)

	if e != nil {
		// Terminate process/thread here.
		d.platformData.terminateAndClose()
		return nil, ec.NewError(e, "InjectDll")
	}

	if !sub.NoJob {
		e = CreateJob(sub, d)
		if e != nil {
			if sub.FailOnJobCreationFailure {
				d.platformData.terminateAndClose()

				return nil, ec.NewError(e, "CreateJob")
			}
			l4g.Error("CreateFrozen/CreateJob: %s", e)
		} else {
			e = win32.AssignProcessToJobObject(d.platformData.hJob, d.platformData.hProcess)
			if e != nil {
				syscall.CloseHandle(d.platformData.hJob)
				d.platformData.hJob = syscall.InvalidHandle
				if sub.FailOnJobCreationFailure {
					d.platformData.terminateAndClose()

					return nil, ec.NewError(e, "AssignProcessToJobObject")
				}
				l4g.Error("CreateFrozen/AssignProcessToJobObject: %s", e)
			}
		}
	}

	if d.platformData.hJob == syscall.InvalidHandle {
		e = win32.SetProcessAffinityMask(d.platformData.hProcess, sub.ProcessAffinityMask)
		if e != nil {
			d.platformData.terminateAndClose()

			return nil, ec.NewError(e, "SetProcessAffinityMask")
		}
	}

	return d, nil
}

func CreateJob(s *Subprocess, d *SubprocessData) error {
	var e error
	ec := tools.NewContext("CreateJob")
	d.platformData.hJob, e = win32.CreateJobObject(nil, nil)
	if e != nil {
		return ec.NewError(e, "CreateJobObject")
	}

	if s.RestrictUi {
		var info win32.JobObjectBasicUiRestrictions
		info.UIRestrictionClass = (win32.JOB_OBJECT_UILIMIT_DESKTOP |
			win32.JOB_OBJECT_UILIMIT_DISPLAYSETTINGS |
			win32.JOB_OBJECT_UILIMIT_EXITWINDOWS |
			win32.JOB_OBJECT_UILIMIT_GLOBALATOMS |
			win32.JOB_OBJECT_UILIMIT_HANDLES |
			win32.JOB_OBJECT_UILIMIT_READCLIPBOARD |
			win32.JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS |
			win32.JOB_OBJECT_UILIMIT_WRITECLIPBOARD)

		e = win32.SetJobObjectBasicUiRestrictions(d.platformData.hJob, &info)
		if e != nil {
			return ec.NewError(e, "SetJobObjectBasicUiRestrictions")
		}
	}

	var einfo win32.JobObjectExtendedLimitInformation
	einfo.BasicLimitInformation.LimitFlags = win32.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION | win32.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	if s.HardTimeLimit > 0 {
		einfo.BasicLimitInformation.PerJobUserTimeLimit = uint64(s.HardTimeLimit.Nanoseconds() / 100)
		einfo.BasicLimitInformation.PerProcessUserTimeLimit = uint64(s.HardTimeLimit.Nanoseconds() / 100)
		einfo.BasicLimitInformation.LimitFlags |= win32.JOB_OBJECT_LIMIT_PROCESS_TIME | win32.JOB_OBJECT_LIMIT_JOB_TIME
	}

	if s.ProcessLimit > 0 {
		einfo.BasicLimitInformation.ActiveProcessLimit = s.ProcessLimit
		einfo.BasicLimitInformation.LimitFlags |= win32.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	}

	if s.HardMemoryLimit > 0 {
		einfo.ProcessMemoryLimit = uintptr(s.HardMemoryLimit)
		einfo.JobMemoryLimit = uintptr(s.HardMemoryLimit)
		einfo.BasicLimitInformation.MaximumWorkingSetSize = uintptr(s.HardMemoryLimit)
		einfo.BasicLimitInformation.LimitFlags |= win32.JOB_OBJECT_LIMIT_JOB_MEMORY | win32.JOB_OBJECT_LIMIT_PROCESS_MEMORY | win32.JOB_OBJECT_LIMIT_WORKINGSET
	}

	// If we don't create job then we need to set process affinity on the process handle after its creation.
	if s.ProcessAffinityMask != 0 {
		einfo.BasicLimitInformation.Affinity = uintptr(s.ProcessAffinityMask)
		einfo.BasicLimitInformation.LimitFlags |= win32.JOB_OBJECT_LIMIT_AFFINITY
	}

	e = win32.SetJobObjectExtendedLimitInformation(d.platformData.hJob, &einfo)
	if e != nil {
		return ec.NewError(e, "SetJobObjectExtendedLimitInformation")
	}
	return nil
}

func InjectDll(s *Subprocess, d *SubprocessData) error {
	if s.Options.InjectDLL == "" || int(s.Options.LoadLibraryW) == 0 {
		return nil
	}

	ec := tools.NewContext("InjectDll")

	l4g.Trace("InjectDll: Injecting library %s with call to %d", s.Options.InjectDLL, s.Options.LoadLibraryW)
	name, err := syscall.UTF16FromString(s.Options.InjectDLL)
	if err != nil {
		return ec.NewError(err, ERR_USER, "UTF16FromString")
	}
	nameLen := uint32((len(name) + 1) * 2)
	remoteName, err := win32.VirtualAllocEx(d.platformData.hProcess, 0, nameLen, win32.MEM_COMMIT, win32.PAGE_READWRITE)
	if err != nil {
		return ec.NewError(os.NewSyscallError("VirtualAllocEx", err))
	}
	defer win32.VirtualFreeEx(d.platformData.hProcess, remoteName, 0, win32.MEM_RELEASE)

	_, err = win32.WriteProcessMemory(d.platformData.hProcess, remoteName, unsafe.Pointer(&name[0]), nameLen)
	if err != nil {
		return ec.NewError(os.NewSyscallError("WriteProcessMemory", err))
	}
	thread, _, err := win32.CreateRemoteThread(d.platformData.hProcess, win32.MakeInheritSa(), 0, s.Options.LoadLibraryW, remoteName, 0)
	if err != nil {
		return ec.NewError(os.NewSyscallError("CreateRemoteThread", err))
	}
	defer syscall.CloseHandle(thread)
	wr, err := syscall.WaitForSingleObject(thread, syscall.INFINITE)
	if err != nil {
		return ec.NewError(os.NewSyscallError("WaitForSingleObject", err))
	}
	if wr != syscall.WAIT_OBJECT_0 {
		return ec.NewError(fmt.Errorf("Unexpected wait result %s", wr))
	}

	return nil
}

func (d *SubprocessData) Unfreeze() error {
	// platform
	hThread := d.platformData.hThread
	win32.ResumeThread(hThread)
	syscall.CloseHandle(hThread)
	return nil
}

func ns100toDuration(ns100 uint64) time.Duration {
	return time.Nanosecond * time.Duration(ns100 * 100)
}

func filetimeToDuration(ft *syscall.Filetime) time.Duration {
	return ns100toDuration(uint64(ft.HighDateTime)<<32 + uint64(ft.LowDateTime))
}

func UpdateProcessTimes(pdata *PlatformData, result *SubprocessResult, finished bool) error {
	creation := &syscall.Filetime{}
	end := &syscall.Filetime{}
	user := &syscall.Filetime{}
	kernel := &syscall.Filetime{}

	err := syscall.GetProcessTimes(pdata.hProcess, creation, end, kernel, user)
	if err != nil {
		return err
	}

	if !finished {
		syscall.GetSystemTimeAsFileTime(end)
	}

	result.WallTime = filetimeToDuration(end) - filetimeToDuration(creation)

	var jinfo *win32.JobObjectBasicAccountingInformation

	if pdata.hJob != syscall.InvalidHandle {
		jinfo, err = win32.GetJobObjectBasicAccountingInformation(pdata.hJob)
		if err != nil {
			l4g.Error(err)
		}
	}

	if jinfo != nil {
		result.UserTime = ns100toDuration(jinfo.TotalUserTime)
		result.KernelTime = ns100toDuration(jinfo.TotalKernelTime)
		result.TotalProcesses = uint64(jinfo.TotalProcesses)
	} else {
		result.UserTime = filetimeToDuration(user)
		result.KernelTime = filetimeToDuration(kernel)
	}

	return nil
}

func GetProcessMemoryUsage(process syscall.Handle) uint64 {
	pmc, err := win32.GetProcessMemoryInfo(process)
	if err != nil {
		return 0
	}

	if pmc.PeakPagefileUsage > pmc.PrivateUsage {
		return uint64(pmc.PeakPagefileUsage)
	}
	return uint64(pmc.PrivateUsage)
}

func UpdateProcessMemory(pdata *PlatformData, result *SubprocessResult) {
	var jinfo *win32.JobObjectExtendedLimitInformation
	var err error

	if pdata.hJob != syscall.InvalidHandle {
		jinfo, err = win32.GetJobObjectExtendedLimitInformation(pdata.hJob)
		if err != nil {
			l4g.Error(err)
		}
	}
	if jinfo != nil {
		result.PeakMemory = uint64(jinfo.PeakJobMemoryUsed)
	} else {
		result.PeakMemory = uint64(GetProcessMemoryUsage(pdata.hProcess))
	}
}

func (sub *Subprocess) BottomHalf(d *SubprocessData, sig chan *SubprocessResult) {
	hProcess := d.platformData.hProcess
	hJob := d.platformData.hJob
	result := &SubprocessResult{}
	var waitResult uint32
	waitResult = syscall.WAIT_TIMEOUT

	var runState runningState

	for result.SuccessCode == 0 && waitResult == syscall.WAIT_TIMEOUT {
		waitResult, _ = syscall.WaitForSingleObject(hProcess, uint32(sub.TimeQuantum.Nanoseconds() / 1000000))
		if waitResult != syscall.WAIT_TIMEOUT {
			break
		}

		_ = UpdateProcessTimes(&d.platformData, result, false)
		if sub.MemoryLimit > 0 {
			UpdateProcessMemory(&d.platformData, result)
		}

		runState.Update(sub, result)
	}

	switch waitResult {
	case syscall.WAIT_OBJECT_0:
		_ = syscall.GetExitCodeProcess(hProcess, &result.ExitCode)

	case syscall.WAIT_TIMEOUT:
		for waitResult == syscall.WAIT_TIMEOUT {
			syscall.TerminateProcess(hProcess, 0)
			waitResult, _ = syscall.WaitForSingleObject(hProcess, 100)
		}
	}

	_ = UpdateProcessTimes(&d.platformData, result, true)
	UpdateProcessMemory(&d.platformData, result)

	syscall.CloseHandle(hProcess)
	if hJob != syscall.InvalidHandle {
		syscall.CloseHandle(hJob)
	}

	sub.SetPostLimits(result)
	for _ = range d.startAfterStart {
		err := <-d.bufferChan
		if err != nil {
			l4g.Error(err)
		}
	}

	if d.stdOut.Len() > 0 {
		result.Output = d.stdOut.Bytes()
	}
	if d.stdErr.Len() > 0 {
		result.Error = d.stdErr.Bytes()
	}

	sig <- result
}

func maybeLockOSThread() {
}

func maybeUnlockOSThread() {
}
