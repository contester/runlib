package subprocess

import (
	l4g "code.google.com/p/log4go"
	"fmt"
	"github.com/contester/runlib/linux"
	"os/user"
	"strconv"
	"syscall"
	"time"
)

type LoginInfo struct {
	Uid int
}

type PlatformOptions struct {
	Cg *linux.Cgroups
}

type PlatformData struct {
	Pid       int
	params    *linux.CloneParams
	startTime time.Time
}

func NewLoginInfo(username, password string) (*LoginInfo, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, NewSubprocessError(false, "NewLoginInfo/user.Lookup", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, NewSubprocessError(false, "NewLoginInfo/strconv.Atoi", err)
	}
	return &LoginInfo{
		Uid: uid,
	}, nil
}

func (d *SubprocessData) wAllRedirects(s *Subprocess, result *linux.StdHandles) error {
	var err error

	if result.StdIn, err = d.SetupInput(s.StdIn); err != nil {
		return err
	}
	if result.StdOut, err = d.SetupOutput(s.StdOut, &d.stdOut); err != nil {
		return err
	}
	if result.StdErr, err = d.SetupOutput(s.StdErr, &d.stdErr); err != nil {
		return err
	}
	return nil
}

func (sub *Subprocess) CreateFrozen() (*SubprocessData, error) {
	if sub.Cmd.ApplicationName == nil {
		return nil, NewSubprocessError(false, "CreateFrozen/init", fmt.Errorf("Application name must be present"))
	}
	d := &SubprocessData{}
	var stdh linux.StdHandles
	err := d.wAllRedirects(sub, &stdh)
	defer stdh.Close()
	if err != nil {
		return nil, NewSubprocessError(false, "CreateFrozen/Redirects", err)
	}
	var uid int
	if sub.Login != nil {
		uid = sub.Login.Uid
	}
	d.platformData.params, err = linux.CreateCloneParams(*sub.Cmd.ApplicationName, sub.Cmd.Parameters, sub.Environment, sub.CurrentDirectory, uid, stdh)
	if err != nil {
		return nil, NewSubprocessError(false, "CreateFrozen/CreateCloneParams", err)
	}
	syscall.ForkLock.Lock()
	d.platformData.Pid, err = d.platformData.params.CloneFrozen()
	closeDescriptors(d.closeAfterStart)
	syscall.ForkLock.Unlock()
	if err != nil {
		return nil, NewSubprocessError(false, "CreateFrozen/CloneFrozen", err)
	}
	err = SetupControlGroup(sub, d)
	if err != nil {
		return nil, NewSubprocessError(false, "CreateFrozen/SetupControlGroup", err)
	}
	return d, nil
}

func SetupControlGroup(s *Subprocess, d *SubprocessData) error {
	cgname := strconv.Itoa(d.platformData.Pid)
	s.Options.Cg.Setup(cgname, d.platformData.Pid)
	return nil
}

func (d *SubprocessData) Unfreeze() error {
	d.platformData.startTime = time.Now()
	return d.platformData.params.Unfreeze(d.platformData.Pid) // TODO: clean
}

type ChildWaitData struct {
	ExitCode                       uint32
	SuccessCode                    uint32
	StopSignal                     uint32
	KillSignal                     uint32
	RusageCpuUser, RusageCpuKernel time.Duration
}

func ChildWaitingFunc(pid int, sig chan *ChildWaitData) {
	var status syscall.WaitStatus
	var rusage syscall.Rusage
	result := &ChildWaitData{}
	for {
		wpid, err := syscall.Wait4(pid, &status, syscall.WUNTRACED|syscall.WCONTINUED, &rusage)
		if wpid != pid {
			continue
		}

		if status.Exited() {
			result.ExitCode = uint32(status.ExitStatus())
			break
		}
		if status.Stopped() {
			result.SuccessCode |= EF_STOPPED
			result.StopSignal = uint32(status.StopSignal())
			syscall.Kill(pid, syscall.SIGKILL)
		}
		if status.Signaled() {
			result.SuccessCode |= EF_KILLED_BY_OTHER
			result.KillSignal = uint32(status.Signal())
			break
		}
		if err != nil {
			break
		}
	}
	result.RusageCpuUser = time.Nanosecond * rusage.Utime.Nano()
	result.RusageCpuKernel = time.Nanosecond * rusage.Stime.Nano()
	sig <- result
	close(sig)
}

func UpdateRunningUsage(p *PlatformData, o *PlatformOptions, result *SubprocessResult) {
	result.WallTime = time.Since(p.startTime)
	result.UserTime = time.Nanosecond * o.Cg.GetCpu(strconv.Itoa(p.Pid))
	result.PeakMemory = o.Cg.GetMemory(strconv.Itoa(p.Pid))
}

func (sub *Subprocess) BottomHalf(d *SubprocessData, sig chan *SubprocessResult) {
	result := &SubprocessResult{}

	childChan := make(chan *ChildWaitData, 1)
	go ChildWaitingFunc(d.platformData.Pid, childChan)
	ticker := time.NewTicker(sub.TimeQuantum)
	var finished *ChildWaitData
	var runState runningState

W:
	for result.SuccessCode == 0 {
		select {
		case finished = <-childChan:
			break W
		case _ = <-ticker.C:
			UpdateRunningUsage(&d.platformData, sub.Options, result)
			runState.Update(sub, result)
		}
	}
	ticker.Stop()
	if finished == nil {
		result.SuccessCode |= EF_KILLED
		syscall.Kill(d.platformData.Pid, syscall.SIGKILL)
		// Can block if process is unkillable.
		finished = <-childChan
	}
	UpdateRunningUsage(&d.platformData, sub.Options, result)
	sub.Options.Cg.Remove(strconv.Itoa(d.platformData.Pid))
	result.ExitCode = finished.ExitCode
	result.KernelTime = finished.RusageCpuKernel
	result.SuccessCode |= finished.SuccessCode
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
	close(sig)
}
