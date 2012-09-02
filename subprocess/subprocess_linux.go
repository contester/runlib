package subprocess

import (
	"fmt"
	"os/user"
	"runlib/linux"
	"strconv"
	"time"
	"syscall"
)

type LoginInfo struct {
	Uid int
}

type PlatformOptions struct{}

type PlatformData struct {
	Pid       int
	params    *linux.CloneParams
	startTime *time.Time
}

func NewLoginInfo(username, password string) (*LoginInfo, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	return &LoginInfo{
		Uid: uid,
	}, nil
}

func (d *subprocessData) wAllRedirects(s *Subprocess, result *linux.StdHandles) error {
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

func (sub *Subprocess) CreateFrozen() (*subprocessData, error) {
	if sub.Cmd.ApplicationName == nil {
		return nil, fmt.Errorf("Application name must be present")
	}
	d := &subprocessData{}
	var stdh linux.StdHandles
	err := d.wAllRedirects(sub, &stdh)
	defer stdh.Close()
	if err != nil {
		return nil, err
	}
	d.platformData.params, err = linux.CreateCloneParams(*sub.Cmd.ApplicationName, sub.Cmd.Parameters, *sub.Environment, *sub.CurrentDirectory, sub.Login.Uid, stdh)
	if err != nil {
		return nil, err
	}
	syscall.ForkLock.Lock()
	d.platformData.Pid, err = d.platformData.params.CloneFrozen()
	closeDescriptors(d.closeAfterStart)
	syscall.ForkLock.Unlock()
	if err != nil {
		return nil, err
	}
	err = SetupControlGroup(sub, d)
	if err != nil {
		return nil, err //TODO: clean up
	}
	return d, nil
}

func SetupControlGroup(s *Subprocess, d *subprocessData) error {
	cgname := strconv.Itoa(d.platformData.Pid)
	linux.CgCreate(cgname)
	linux.CgAttach(cgname, d.platformData.Pid)
	return nil
}

func (d *subprocessData) Unfreeze() error {
	return d.platformData.params.Unfreeze(d.platformData.Pid) // TODO: clean
}

type ChildWaitData struct {
	ExitCode                       uint32
	SuccessCode uint32
	StopSignal                     uint32
	KillSignal                     uint32
	RusageCpuUser, RusageCpuKernel uint64
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
			result.StopSignal = status.StopSignal()
			syscall.Kill(pid, syscall.SIGKILL)
		}
		if status.Signaled() {
			result.SuccessCode |= EF_KILLED_BY_OTHER
			result.KillSignal = status.Signal()
			break
		}
	}
	result.RusageCpuUser = rusage.Utime.Nano()
	result.RusageCpuKernel = rusage.Stime.Nano()
	sig <- result
	close(sig)
}

func UpdateWallTime(p *PlatformData, result *SubprocessResult) {
	result.WallTime = time.Since(p.startTime).Nanoseconds() / 1000
}

func UpdateContainerTime(p *PlatformData, result *SubprocessResult) {
	result.UserTime = linux.CgGetCpu(strconv.Itoa(p.Pid))
}

func UpdateContainerMemory(p *PlatformData, result *SubprocessResult) {
	result.PeakMemory = linux.CgGetMemory(strconv.Itoa(p.Pid))
}

func UpdateRunningUsage(p *PlatformData, result *SubprocessResult) {
	UpdateWallTime(p, result)
	UpdateContainerTime(p, result)
	UpdateContainerMemory(p, result)
}

func (sub *Subprocess) BottomHalf(d *subprocessData, sig chan *SubprocessResult) {
	result = &SubprocessResult{}

	childChan := make(chan *ChildWaitData)
	go ChildWaitingFunc(d.platformData.Pid, childChan)
	ticker := time.NewTicker(time.Second / 4)
	var finished *ChildWaitData
	var memlimitHit, timelimitHit bool
	var ttLast uint64
	for result.SuccessCode == 0 {
		select {
		case finished = <-childChan:
			break
		case tick := <-ticker.C:
			UpdateRunningUsage(d.platformData, result)
			ttLastNew := result.KernelTime + result.UserTime
			if sub.CheckIdleness && (ttLastNew == ttLast) {
				result.SuccessCode |= EF_INACTIVE
			}
			ttLast = ttLastNew
			if sub.TimeLimit > 0 && (result.UserTime > sub.TimeLimit) {
				result.SuccessCode |= EF_TIME_LIMIT_HIT
			}
			if sub.HardTimeLimit > 0 && (result.WallTime > sub.HardTimeLimit) {
				result.SuccessCode |= EF_TIME_LIMIT_HARD
			}

			if sub.MemoryLimit > 0 && (result.PeakMemory > sub.MemoryLimit) {
				result.SuccessCode |= EF_MEMORY_LIMIT_HIT
			}
		}
	}
	if finished == nil {
		result.SuccessCode |= EF_KILLED
		syscall.Kill(d.platformData.Pid, SIGKILL)
		finished = <-childChan
	}
	UpdateRunningUsage(d.platformData, result)
	result.ExitCode = finished.ExitCode
	result.KernelTime = finished.RusageCpuKernel
	result.SuccessCode |= finished.SuccessCode

	if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
		result.SuccessCode |= EF_TIME_LIMIT_HIT_POST
	}

	if (sub.MemoryLimit > 0) && (result.PeakMemory > sub.MemoryLimit) {
		result.SuccessCode |= EF_MEMORY_LIMIT_HIT_POST
	}

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
