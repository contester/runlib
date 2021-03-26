package subprocess

import (
	"bytes"
	"io"
	"time"
)

const (
	EF_INACTIVE                   = (1 << 0)
	EF_TIME_LIMIT_HIT             = (1 << 1)
	EF_MEMORY_LIMIT_HIT           = (1 << 3)
	EF_KILLED                     = (1 << 4)
	EF_STDOUT_OVERFLOW            = (1 << 5)
	EF_STDERR_OVERFLOW            = (1 << 6)
	EF_STDPIPE_TIMEOUT            = (1 << 7)
	EF_TIME_LIMIT_HIT_POST        = (1 << 8)
	EF_MEMORY_LIMIT_HIT_POST      = (1 << 9)
	EF_PROCESS_LIMIT_HIT          = (1 << 10)
	EF_PROCESS_LIMIT_HIT_POST     = (1 << 11)
	EF_STOPPED                    = (1 << 12)
	EF_KILLED_BY_OTHER            = (1 << 13)
	EF_KERNEL_TIME_LIMIT_HIT      = (1 << 15)
	EF_KERNEL_TIME_LIMIT_HIT_POST = (1 << 16)
	EF_WALL_TIME_LIMIT_HIT        = (1 << 2)
	EF_WALL_TIME_LIMIT_HIT_POST   = (1 << 14)

	REDIRECT_NONE   = 0
	REDIRECT_MEMORY = 1
	REDIRECT_FILE   = 2
	REDIRECT_PIPE   = 3
	REDIRECT_REMOTE = 4
)

func GetMicros(d time.Duration) uint64 {
	return uint64(d / time.Microsecond)
}

func DuFromMicros(ms uint64) time.Duration {
	return time.Microsecond * time.Duration(ms)
}

type TimeStats struct {
	UserTime, KernelTime, WallTime time.Duration
}

type SubprocessResult struct {
	SuccessCode uint32
	ExitCode    uint32
	TimeStats
	PeakMemory     uint64
	TotalProcesses uint64

	Output []byte
	Error  []byte
}

type CommandLine struct {
	ApplicationName, CommandLine string
	Parameters                   []string
}

// This structure defines all flags and options for starting a subprocess. It is not supposed to be modified by any
// of the execution machinery - there's SubprocessData for that.
type Subprocess struct {
	CurrentDirectory string
	Environment      []string

	NoInheritEnvironment     bool
	NoJob                    bool
	RestrictUi               bool
	ProcessLimit             uint32
	FailOnJobCreationFailure bool

	TimeLimit           time.Duration
	KernelTimeLimit     time.Duration
	WallTimeLimit       time.Duration
	CheckIdleness       bool
	MemoryLimit         uint64
	HardMemoryLimit     uint64
	TimeQuantum         time.Duration
	ProcessAffinityMask uint64

	Cmd                   *CommandLine
	Login                 *LoginInfo
	StdIn, StdOut, StdErr *Redirect
	JoinStdOutErr         bool

	Options *PlatformOptions
}

// State for the running subprocess.
type SubprocessData struct {
	bufferChan      chan error     // receives buffer errors
	startAfterStart []func() error // buffer functions, launch after createFrozen
	closeAfterStart []io.Closer    // close after createFrozen
	cleanupIfFailed []func()

	stdOut bytes.Buffer
	stdErr bytes.Buffer

	platformData PlatformData
}

func SubprocessCreate() *Subprocess {
	return &Subprocess{
		TimeQuantum: time.Second / 4,
	}
}

func (d *SubprocessData) SetupRedirectionBuffers() {
	d.bufferChan = make(chan error, len(d.startAfterStart))

	for _, fn := range d.startAfterStart {
		go func(fn func() error) {
			d.bufferChan <- fn()
		}(fn)
	}
}

func closeDescriptors(closers []io.Closer) {
	for _, fd := range closers {
		fd.Close()
	}
}

func (sub *Subprocess) Execute() (*SubprocessResult, error) {
	// Locking of the OS thread is needed on linux, because PtraceDetach will not work if you do it from the
	// different thread.
	maybeLockOSThread()
	defer maybeUnlockOSThread()

	d, err := sub.CreateFrozen()
	if err != nil {
		if d != nil {
			for _, v := range d.cleanupIfFailed {
				v()
			}
		}
		return nil, err
	}

	d.SetupRedirectionBuffers()
	d.Unfreeze()
	return sub.BottomHalf(d), nil
}

type runningState struct {
	lastTimeUsed    time.Duration
	noTimeUsedCount uint
}

func (r *runningState) Update(sub *Subprocess, result *SubprocessResult) {
	ttLastNew := result.KernelTime + result.UserTime

	if ttLastNew == r.lastTimeUsed {
		r.noTimeUsedCount++
	} else {
		r.noTimeUsedCount = 0
	}

	if sub.CheckIdleness && (r.noTimeUsedCount >= 6) && (result.WallTime > sub.TimeLimit) {
		result.SuccessCode |= EF_INACTIVE
	}

	if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
		result.SuccessCode |= EF_TIME_LIMIT_HIT
	}

	if (sub.KernelTimeLimit > 0) && (result.KernelTime > sub.KernelTimeLimit) {
		result.SuccessCode |= EF_KERNEL_TIME_LIMIT_HIT
	}

	if (sub.WallTimeLimit > 0) && (result.WallTime > sub.WallTimeLimit) {
		result.SuccessCode |= EF_WALL_TIME_LIMIT_HIT
	}

	r.lastTimeUsed = ttLastNew

	if (sub.MemoryLimit > 0) && (result.PeakMemory > sub.MemoryLimit) {
		result.SuccessCode |= EF_MEMORY_LIMIT_HIT
	}
}

func (sub *Subprocess) SetPostLimits(result *SubprocessResult) {
	if (sub.TimeLimit > 0) && (result.UserTime > sub.TimeLimit) {
		result.SuccessCode |= EF_TIME_LIMIT_HIT_POST
	}

	if (sub.MemoryLimit > 0) && (result.PeakMemory > sub.MemoryLimit) {
		result.SuccessCode |= EF_MEMORY_LIMIT_HIT_POST
	}

	if (sub.KernelTimeLimit > 0) && (result.KernelTime > sub.KernelTimeLimit) {
		result.SuccessCode |= EF_KERNEL_TIME_LIMIT_HIT_POST
	}
}
