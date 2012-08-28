package subprocess

// +build windows,386

import (
	"bytes"
	l4g "code.google.com/p/log4go"
	"runlib/platform/win32"
	"syscall"
	"unsafe"
	"fmt"
)


type PlatformData struct {
	hProcess syscall.Handle
	hThread  syscall.Handle
	hJob     syscall.Handle
}

type PlatformOptions struct {
	Desktop string
	InjectDLL string
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

func (d *subprocessData) wOutputRedirect(w *Redirect, b *bytes.Buffer) (syscall.Handle, error) {
	f, err := d.SetupOutput(w, b)
	if err != nil || f == nil {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(f.Fd()), nil
}

func (d *subprocessData) wInputRedirect(w *Redirect) (syscall.Handle, error) {
	f, err := d.SetupInput(w)
	if err != nil || f == nil {
		return syscall.InvalidHandle, err
	}
	return syscall.Handle(f.Fd()), nil
}

func (d *subprocessData) wAllRedirects(s *Subprocess, si *syscall.StartupInfo) error {
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

func (sub *Subprocess) CreateFrozen() (*subprocessData, error) {
	d := &subprocessData{}

	si := &syscall.StartupInfo{}
	si.Cb = uint32(unsafe.Sizeof(*si))
	si.Flags = win32.STARTF_FORCEOFFFEEDBACK | syscall.STARTF_USESHOWWINDOW
	si.ShowWindow = syscall.SW_SHOWMINNOACTIVE
	if !sub.NoJob && sub.Options != nil && sub.Options.Desktop != "" {
		si.Desktop = syscall.StringToUTF16Ptr(sub.Options.Desktop)
	}
	d.wAllRedirects(sub, si)

	pi := &syscall.ProcessInformation{}

	applicationName := win32.StringPtrToUTF16Ptr(sub.Cmd.ApplicationName)
	commandLine := win32.StringPtrToUTF16Ptr(sub.Cmd.CommandLine)
	environment := win32.ListToEnvironmentBlock(sub.Environment)
	currentDirectory := win32.StringPtrToUTF16Ptr(sub.CurrentDirectory)

	var e error

	if sub.Login != nil {
		if sub.NoJob {
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
			e = win32.CreateProcessAsUser(
				sub.Login.HUser,
				applicationName,
				commandLine,
				nil,
				nil,
				true,
				win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
				environment,
				currentDirectory,
				si,
				pi)
		}
	} else {
		e = syscall.CreateProcess(
			applicationName,
			commandLine,
			nil,
			nil,
			true,
			win32.CREATE_NEW_PROCESS_GROUP|win32.CREATE_NEW_CONSOLE|win32.CREATE_SUSPENDED|syscall.CREATE_UNICODE_ENVIRONMENT|win32.CREATE_BREAKAWAY_FROM_JOB,
			environment,
			currentDirectory,
			si,
			pi)
	}

	if e != nil {
		return nil, e
	}

	d.platformData.hProcess = pi.Process
	d.platformData.hThread = pi.Thread
	d.platformData.hJob = syscall.InvalidHandle

	e = InjectDll(sub, d)

	if e != nil {
		l4g.Error(e)
	}

	if !sub.NoJob {
		d.platformData.hJob, e = win32.CreateJobObject(nil, nil)
		if e != nil {
			l4g.Error(e)
		} else {
			e = win32.AssignProcessToJobObject(d.platformData.hJob, d.platformData.hProcess)
			if e != nil {
				l4g.Error(e)
				syscall.CloseHandle(d.platformData.hJob)
				d.platformData.hJob = syscall.InvalidHandle
			}
		}
	}

	return d, e
}

func InjectDll(s *Subprocess, d *subprocessData) error {
	if s.Options.InjectDLL == "" || int(s.Options.LoadLibraryW) == 0 {
		return nil
	}

	l4g.Info("Injecting", s.Options.InjectDLL, s.Options.LoadLibraryW)
	name := syscall.StringToUTF16(s.Options.InjectDLL)
	nameLen := uint32((len(name) + 1) * 2)
	remoteName, err := win32.VirtualAllocEx(d.platformData.hProcess, 0, nameLen, win32.MEM_COMMIT, win32.PAGE_READWRITE)
	if err != nil {
		return err
	}
	defer win32.VirtualFreeEx(d.platformData.hProcess, remoteName, 0, win32.MEM_RELEASE)

	_, err = win32.WriteProcessMemory(d.platformData.hProcess, remoteName, unsafe.Pointer(&name[0]), nameLen)
	if err != nil {
		return err
	}
	thread, _, err := win32.CreateRemoteThread(d.platformData.hProcess, win32.MakeInheritSa(), 0, s.Options.LoadLibraryW, remoteName, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(thread)
	wr, err := syscall.WaitForSingleObject(thread, syscall.INFINITE)
	if err != nil {
		return err
	}
	if wr != syscall.WAIT_OBJECT_0 {
		return fmt.Errorf("foo")
	}
	
	return nil
}

func (d *subprocessData) SetupOnFrozen() error {
	// portable
	closeDescriptors(d.closeAfterStart)

	d.bufferChan = make(chan error, len(d.startAfterStart))

	for _, fn := range d.startAfterStart {
		go func(fn func() error) {
			d.bufferChan <- fn()
		}(fn)
	}

	return nil
}

func (d *subprocessData) Unfreeze() error {
	// platform
	hThread := d.platformData.hThread
	win32.ResumeThread(hThread)
	syscall.CloseHandle(hThread)
	return nil
}

func FiletimeToUint64(ft *syscall.Filetime) uint64 {
	return uint64(ft.HighDateTime)<<32 + uint64(ft.LowDateTime)
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

	result.WallTime = (FiletimeToUint64(end) / 10) - (FiletimeToUint64(creation) / 10)

	var jinfo *win32.JobObjectBasicAccountingInformation

	if pdata.hJob != syscall.InvalidHandle {
		jinfo, err = win32.GetJobObjectBasicAccountingInformation(pdata.hJob)
		if err != nil {
			l4g.Error(err)
		}
	}

	if jinfo != nil {
		result.UserTime = jinfo.TotalUserTime / 10
		result.KernelTime = jinfo.TotalKernelTime / 10
		result.TotalProcesses = uint64(jinfo.TotalProcesses)
	} else {
		result.UserTime = FiletimeToUint64(user) / 10
		result.KernelTime = FiletimeToUint64(kernel) / 10
	}

	return nil
}

func GetProcessMemoryUsage(process syscall.Handle) uint32 {
	pmc, err := win32.GetProcessMemoryInfo(process)
	if err != nil {
		return 0
	}

	if pmc.PeakPagefileUsage > pmc.PrivateUsage {
		return pmc.PeakPagefileUsage
	}
	return pmc.PrivateUsage
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

func (sub *Subprocess) BottomHalf(d *subprocessData, sig chan *SubprocessResult) {
	hProcess := d.platformData.hProcess
	hJob := d.platformData.hJob
	result := &SubprocessResult{}
	var waitResult uint32
	waitResult = syscall.WAIT_TIMEOUT
	var ttLast uint64
	ttLast = 0

	for result.SuccessCode == 0 && waitResult == syscall.WAIT_TIMEOUT {
		waitResult, _ = syscall.WaitForSingleObject(hProcess, sub.TimeQuantum)
		if waitResult != syscall.WAIT_TIMEOUT {
			break
		}

		_ = UpdateProcessTimes(&d.platformData, result, false)
		ttLastNew := result.KernelTime + result.UserTime

		if sub.CheckIdleness && (ttLast == ttLastNew) {
			result.SuccessCode |= EF_INACTIVE
		}

		if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
			result.SuccessCode |= EF_TIME_LIMIT_HIT
		}

		if (sub.HardTimeLimit > 0) && (result.WallTime > sub.HardTimeLimit) {
			result.SuccessCode |= EF_TIME_LIMIT_HARD
		}

		ttLast = ttLastNew

		if sub.MemoryLimit > 0 {
			UpdateProcessMemory(&d.platformData, result)
			if result.PeakMemory > sub.MemoryLimit {
				result.SuccessCode |= EF_MEMORY_LIMIT_HIT
			}
		}
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

	if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
		result.SuccessCode |= EF_TIME_LIMIT_HIT_POST
	}

	if (sub.MemoryLimit > 0) && (result.PeakMemory > sub.MemoryLimit) {
		result.SuccessCode |= EF_MEMORY_LIMIT_HIT_POST
	}
	// log.Println("Must collect", len(d.startAfterStart), "redirects")
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

func (sub *Subprocess) Execute() (*SubprocessResult, error) {
	d, err := sub.CreateFrozen()
	if err != nil {
		return nil, err
	}

	if err = d.SetupOnFrozen(); err != nil {
		return nil, err // we must die here
	}

	sig := make(chan *SubprocessResult)
	go sub.BottomHalf(d, sig)

	if err = d.Unfreeze(); err != nil {
		return nil, err
	}

	return <-sig, nil
}
