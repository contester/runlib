package subprocess

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/contester/runlib/win32"

	log "github.com/sirupsen/logrus"
)

type PlatformData struct {
	hProcess syscall.Handle
	hThread  syscall.Handle
	hJob     syscall.Handle

	hStdIn  syscall.Handle
	hStdOut syscall.Handle
	hStdErr syscall.Handle

	use32BitLoadLibrary bool
}

type PlatformOptions struct {
	Environment PlatformEnvironment
	InjectDLL   []string
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

func (d *SubprocessData) wOutputRedirect(w *Redirect, b *bytes.Buffer, isStdErr bool) (syscall.Handle, error) {
	f, err := d.SetupOutput(w, b, isStdErr)
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
	if si.StdOutput, err = d.wOutputRedirect(s.StdOut, &d.stdOut, false); err != nil {
		return err
	}
	if s.JoinStdOutErr {
		si.StdErr = si.StdOutput
	} else {
		if si.StdErr, err = d.wOutputRedirect(s.StdErr, &d.stdErr, true); err != nil {
			return err
		}
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
	var d SubprocessData

	si := syscall.StartupInfo{
		Flags:      win32.STARTF_FORCEOFFFEEDBACK | syscall.STARTF_USESHOWWINDOW,
		ShowWindow: syscall.SW_SHOWMINNOACTIVE,
	}
	si.Cb = uint32(unsafe.Sizeof(si))
	useCreateProcessWithLogonW := sub.NoJob || win32.IsWindows8OrGreater()

	if sub.Options != nil && sub.Options.Environment != nil {
		if err := d.initArchDependentData(sub); err != nil {
			return nil, err
		}

		if !useCreateProcessWithLogonW {
			desktopName, err := sub.Options.Environment.GetDesktopName()
			if err != nil {
				return nil, err
			}

			si.Desktop = syscall.StringToUTF16Ptr(desktopName)
		}
	}

	e := d.wAllRedirects(sub, &si)
	if e != nil {
		return nil, e
	}

	var pi syscall.ProcessInformation

	syscall.ForkLock.Lock()
	wSetInherit(&si)

	if sub.Login != nil {
		if useCreateProcessWithLogonW {
			e = win32.CreateProcessWithLogonW(
				sub.Login.Username,
				".",
				sub.Login.Password,
				win32.LOGON_WITH_PROFILE,
				sub.Cmd.ApplicationName,
				sub.Cmd.CommandLine,
				win32.CREATE_SUSPENDED|syscall.CREATE_UNICODE_ENVIRONMENT,
				win32.ProcessEnvironmentOptions{
					NoInherit: sub.NoInheritEnvironment,
					Env:       sub.Environment,
				},
				sub.CurrentDirectory,
				&si,
				&pi)
		} else {
			e = win32.CreateProcessAsUser(
				sub.Login.HUser,
				sub.Cmd.ApplicationName,
				sub.Cmd.CommandLine,
				nil,
				nil,
				true,
				win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|
					syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
				win32.ProcessEnvironmentOptions{
					NoInherit: sub.NoInheritEnvironment,
					Env:       sub.Environment,
				},
				sub.CurrentDirectory,
				&si,
				&pi)
		}
	} else {
		e = win32.CreateProcess(
			sub.Cmd.ApplicationName,
			sub.Cmd.CommandLine,
			nil,
			nil,
			true,
			win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|
				syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
			win32.ProcessEnvironmentOptions{
				NoInherit: sub.NoInheritEnvironment,
				Env:       sub.Environment,
			},
			sub.CurrentDirectory,
			&si,
			&pi)
	}

	closeDescriptors(d.closeAfterStart)
	syscall.ForkLock.Unlock()

	if e != nil {
		if errno, ok := extractErrno(e); ok && errno == 136 {
			e = fmt.Errorf("%w: CreateProcess(%q): errno 136: %w", sub.Cmd.ApplicationName, ErrUserError, e)
		} else {
			e = fmt.Errorf("CreateProcess(%q): %w", sub.Cmd.ApplicationName, e)
		}
		return nil, e
	}

	d.platformData.hProcess = pi.Process
	d.platformData.hThread = pi.Thread
	d.platformData.hJob = syscall.InvalidHandle

	for _, dll := range sub.Options.InjectDLL {
		if e = InjectDll(&d, sub.Options.Environment, dll); e != nil {
			break
		}
	}

	if e != nil {
		// Terminate process/thread here.
		d.platformData.terminateAndClose()
		return nil, e
	}

	if sub.ProcessAffinityMask != 0 {
		e = win32.SetProcessAffinityMask(d.platformData.hProcess, sub.ProcessAffinityMask)
		if e != nil {
			d.platformData.terminateAndClose()
			return nil, fmt.Errorf("SetProcessAffinityMask(b%b): %w", sub.ProcessAffinityMask, e)
		}
	}

	if !sub.NoJob {
		e = CreateJob(sub, &d)
		if e != nil {
			if sub.FailOnJobCreationFailure {
				d.platformData.terminateAndClose()

				return nil, fmt.Errorf("CreateJob: %w", e)
			}
			log.Error("CreateFrozen/CreateJob: %s", e)
		} else {
			e = win32.AssignProcessToJobObject(d.platformData.hJob, d.platformData.hProcess)
			if e != nil {
				log.Errorf("CreateFrozen/AssignProcessToJobObject: %s, hJob: %d, hProcess: %d, pd: %+v", e,
					d.platformData.hJob, d.platformData.hProcess, d.platformData)
				syscall.CloseHandle(d.platformData.hJob)
				d.platformData.hJob = syscall.InvalidHandle
				if sub.FailOnJobCreationFailure {
					d.platformData.terminateAndClose()

					return nil, fmt.Errorf("AssignProcessToJobObject: %w", e)
				}
			}
		}
	}

	return &d, nil
}

func CreateJob(s *Subprocess, d *SubprocessData) error {
	var e error
	d.platformData.hJob, e = win32.CreateJobObject(nil, nil)
	if e != nil {
		return fmt.Errorf("CreateJobObject: %w", e)
	}

	if s.RestrictUi {
		info := win32.JobObjectBasicUiRestrictions{
			UIRestrictionClass: (win32.JOB_OBJECT_UILIMIT_DESKTOP |
				win32.JOB_OBJECT_UILIMIT_DISPLAYSETTINGS |
				win32.JOB_OBJECT_UILIMIT_EXITWINDOWS |
				win32.JOB_OBJECT_UILIMIT_GLOBALATOMS |
				win32.JOB_OBJECT_UILIMIT_HANDLES |
				win32.JOB_OBJECT_UILIMIT_READCLIPBOARD |
				win32.JOB_OBJECT_UILIMIT_SYSTEMPARAMETERS |
				win32.JOB_OBJECT_UILIMIT_WRITECLIPBOARD),
		}

		if e = win32.SetJobObjectBasicUiRestrictions(d.platformData.hJob, &info); e != nil {
			syscall.CloseHandle(d.platformData.hJob)
			return fmt.Errorf("SetJobObjectBasicUiRestrictions: %w", e)
		}
	}

	einfo := win32.JobObjectExtendedLimitInformation{
		BasicLimitInformation: win32.JobObjectBasicLimitInformation{
			LimitFlags: win32.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION | win32.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}

	var hardTimeLimit time.Duration
	if s.TimeLimit > 0 {
		hardTimeLimit = s.TimeLimit + time.Second
	} else {
		hardTimeLimit = s.WallTimeLimit
	}

	if hardTimeLimit > 0 {
		log.Debugf("Setting hard limits on time: %s", hardTimeLimit)
		nsLimit := uint64(hardTimeLimit.Nanoseconds() / 100)
		einfo.BasicLimitInformation.PerJobUserTimeLimit = nsLimit
		einfo.BasicLimitInformation.PerProcessUserTimeLimit = nsLimit
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
	// if s.ProcessAffinityMask != 0 {
	//	einfo.BasicLimitInformation.Affinity = uintptr(s.ProcessAffinityMask)
	//	einfo.BasicLimitInformation.LimitFlags |= win32.JOB_OBJECT_LIMIT_AFFINITY
	//}

	e = win32.SetJobObjectExtendedLimitInformation(d.platformData.hJob, &einfo)
	if e != nil {
		syscall.CloseHandle(d.platformData.hJob)
		return fmt.Errorf("SetJobObjectExtendedLimitInformation: %w", e)
	}
	return nil
}

func InjectDll(d *SubprocessData, env PlatformEnvironment, dll string) error {
	loadLibraryW, err := d.getLoadLibraryW(env)
	if err != nil {
		return err
	}
	if int(loadLibraryW) == 0 {
		return nil
	}

	log.Debug("InjectDll: Injecting library %s with call to %d", dll, loadLibraryW)
	name, err := syscall.UTF16FromString(dll)
	if err != nil {
		return fmt.Errorf("%w: UTF16FromString(%q): %w", ErrUserError, dll, err)
	}
	nameLen := uint32((len(name) + 1) * 2)
	remoteName, err := win32.VirtualAllocEx(d.platformData.hProcess, 0, nameLen, win32.MEM_COMMIT, win32.PAGE_READWRITE)
	if err != nil {
		return fmt.Errorf("VirtualAllocEx(%d): %w", nameLen, err)
	}
	defer win32.VirtualFreeEx(d.platformData.hProcess, remoteName, 0, win32.MEM_RELEASE)

	if _, err = win32.WriteProcessMemory(d.platformData.hProcess, remoteName, unsafe.Pointer(&name[0]), nameLen); err != nil {
		return fmt.Errorf("WriteProcessMemory: %w", err)
	}
	runtime.KeepAlive(name)
	thread, _, err := win32.CreateRemoteThread(d.platformData.hProcess, win32.MakeInheritSa(), 0, loadLibraryW, remoteName, 0)
	if err != nil {
		return fmt.Errorf("CreateRemoteThread: %w", err)
	}
	defer syscall.CloseHandle(thread)
	wr, err := syscall.WaitForSingleObject(thread, syscall.INFINITE)
	if err != nil {
		return fmt.Errorf("WaitForSingleObject: %w", os.NewSyscallError("WaitForSingleObject", err))
	}
	if wr != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("Unexpected wait result %v", wr)
	}

	return nil
}

func (d *SubprocessData) Unfreeze() error {
	hThread := d.platformData.hThread
	defer syscall.CloseHandle(hThread)
	var err error
	retries := 10
	for {
		var oldCount int
		retries--
		oldCount, err = win32.ResumeThread(hThread)
		if oldCount <= 1 && err == nil {
			break
		}
		log.Errorf("unfreeze: oldcount %d, error %s", oldCount, err)
		if retries <= 0 {
			// crash
			log.Fatalf("UNSUSPEND FAILED, CRASHING")
		}
		time.Sleep(time.Second / 10)
	}
	return nil
}

func ns100toDuration(ns100 uint64) time.Duration {
	return time.Nanosecond * time.Duration(ns100*100)
}

func filetimeToDuration(ft *syscall.Filetime) time.Duration {
	return ns100toDuration(uint64(ft.HighDateTime)<<32 + uint64(ft.LowDateTime))
}

func UpdateProcessTimes(pdata *PlatformData, result *SubprocessResult, finished bool) error {
	var creation, end, user, kernel syscall.Filetime

	err := syscall.GetProcessTimes(pdata.hProcess, &creation, &end, &kernel, &user)
	if err != nil {
		return err
	}

	if !finished {
		syscall.GetSystemTimeAsFileTime(&end)
	}

	result.WallTime = filetimeToDuration(&end) - filetimeToDuration(&creation)

	var jinfo *win32.JobObjectBasicAccountingInformation

	if pdata.hJob != syscall.InvalidHandle {
		jinfo, err = win32.GetJobObjectBasicAccountingInformation(pdata.hJob)
		if err != nil {
			log.Error(err)
		}
	}

	if jinfo != nil {
		result.UserTime = ns100toDuration(jinfo.TotalUserTime)
		result.KernelTime = ns100toDuration(jinfo.TotalKernelTime)
		result.TotalProcesses = uint64(jinfo.TotalProcesses)
	} else {
		result.UserTime = filetimeToDuration(&user)
		result.KernelTime = filetimeToDuration(&kernel)
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
			log.Error(err)
		}
	}
	if jinfo != nil {
		result.PeakMemory = uint64(jinfo.PeakJobMemoryUsed)
	} else {
		result.PeakMemory = uint64(GetProcessMemoryUsage(pdata.hProcess))
	}
}

func loopTerminate(hProcess syscall.Handle) {
	for {
		if err := syscall.TerminateProcess(hProcess, 0); err != nil {
			log.Errorf("Error terminating process %d: %s", hProcess, err)
		}
		waitResult, err := syscall.WaitForSingleObject(hProcess, 1000)
		if err != nil {
			log.Errorf("Error waiting for kill %d: %s", hProcess, err)
		} else if waitResult != syscall.WAIT_TIMEOUT {
			break
		}
	}
}

func (sub *Subprocess) BottomHalf(d *SubprocessData) *SubprocessResult {
	hProcess := d.platformData.hProcess
	hJob := d.platformData.hJob
	var result SubprocessResult
	var waitResult uint32
	waitResult = syscall.WAIT_TIMEOUT

	var runState runningState
	var err error

	for result.SuccessCode == 0 && waitResult == syscall.WAIT_TIMEOUT {
		waitResult, err = syscall.WaitForSingleObject(hProcess, uint32(sub.TimeQuantum.Milliseconds()))
		if waitResult != syscall.WAIT_TIMEOUT {
			break
		}
		if err != nil {
			log.Errorf("Error waiting for process %d: %s", hProcess, err)
		}

		if err = UpdateProcessTimes(&d.platformData, &result, false); err != nil {
			log.Errorf("Error getting process times: %s", err)
		}
		if sub.MemoryLimit > 0 {
			UpdateProcessMemory(&d.platformData, &result)
		}

		runState.Update(sub, &result)

		if d.outCheck != nil {
			err = d.outCheck.Check()
			if err != nil {
				result.OutputLimitExceeded = true
				result.SuccessCode |= EF_STDOUT_OVERFLOW
				break
			}
		}

		if d.errCheck != nil {
			err = d.errCheck.Check()
			if err != nil {
				result.ErrorLimitExceeded = true
				result.SuccessCode |= EF_STDERR_OVERFLOW
				break
			}
		}
	}

	if err != nil {
		loopTerminate(hProcess)
	} else {
		switch waitResult {
		case syscall.WAIT_OBJECT_0:
			if err = syscall.GetExitCodeProcess(hProcess, &result.ExitCode); err != nil {
				log.Errorf("Error getting exit code %d: %s", hProcess, err)
			}

		case syscall.WAIT_TIMEOUT:
			loopTerminate(hProcess)
		default:
			log.Errorf("Unexpected waitResult %d: %d", hProcess, waitResult)
		}
	}

	UpdateProcessTimes(&d.platformData, &result, true)
	UpdateProcessMemory(&d.platformData, &result)

	syscall.CloseHandle(hProcess)
	if hJob != syscall.InvalidHandle {
		syscall.CloseHandle(hJob)
	}

	sub.SetPostLimits(&result)
	for range d.startAfterStart {
		err := <-d.bufferChan
		if err != nil {
			log.Error(err)
		}
	}

	if d.stdOut.Len() > 0 {
		result.Output = d.stdOut.Bytes()
	}
	if d.stdErr.Len() > 0 {
		result.Error = d.stdErr.Bytes()
	}

	if d.errCheck != nil {
		d.errCheck.Close()
	}
	if d.outCheck != nil {
		d.outCheck.Close()
	}

	return &result
}

func maybeLockOSThread() {
}

func maybeUnlockOSThread() {
}
