package subprocess

import (
	"bytes"
	"io"
	"time"
)

const (
	EF_INACTIVE               = (1 << 0)
	EF_TIME_LIMIT_HIT         = (1 << 1)
	EF_TIME_LIMIT_HARD        = (1 << 2)
	EF_MEMORY_LIMIT_HIT       = (1 << 3)
	EF_KILLED                 = (1 << 4)
	EF_STDOUT_OVERFLOW        = (1 << 5)
	EF_STDERR_OVERFLOW        = (1 << 6)
	EF_STDPIPE_TIMEOUT        = (1 << 7)
	EF_TIME_LIMIT_HIT_POST    = (1 << 8)
	EF_MEMORY_LIMIT_HIT_POST  = (1 << 9)
	EF_PROCESS_LIMIT_HIT      = (1 << 10)
	EF_PROCESS_LIMIT_HIT_POST = (1 << 11)
	EF_STOPPED                = (1 << 12)
	EF_KILLED_BY_OTHER        = (1 << 13)

	REDIRECT_NONE   = 0
	REDIRECT_MEMORY = 1
	REDIRECT_FILE   = 2
	REDIRECT_PIPE   = 3
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
	HardTimeLimit       time.Duration
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

	stdOut bytes.Buffer
	stdErr bytes.Buffer

	platformData PlatformData
}

func SubprocessCreate() *Subprocess {
	return &Subprocess{
		TimeQuantum: time.Second / 4,
	}
}

func (d *SubprocessData) SetupRedirectionBuffers() error {
	// portable
	d.bufferChan = make(chan error, len(d.startAfterStart))

	for _, fn := range d.startAfterStart {
		go func(fn func() error) {
			d.bufferChan <- fn()
		}(fn)
	}

	return nil
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
		return nil, err
	}

	if err = d.SetupRedirectionBuffers(); err != nil {
		return nil, err // we must die here
	}

	d.Unfreeze()
	sig := make(chan *SubprocessResult, 1)
	go sub.BottomHalf(d, sig)

	//if err = d.Unfreeze(); err != nil {
	//	return nil, err
	//}

	return <-sig, nil
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

	if (sub.HardTimeLimit > 0) && (result.WallTime > sub.HardTimeLimit) {
		result.SuccessCode |= EF_TIME_LIMIT_HARD
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
}
