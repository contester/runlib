package subprocess

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

	REDIRECT_NONE   = 0
	REDIRECT_MEMORY = 1
	REDIRECT_FILE   = 2
	REDIRECT_HANDLE = 3
)

type SubprocessResult struct {
	SuccessCode    uint32
	ExitCode       uint32
	UserTime       uint64
	KernelTime     uint64
	WallTime       uint64
	PeakMemory     uint64
	TotalProcesses uint64

	Output []byte
	Error  []byte
}

type CommandLine struct {
	ApplicationName, CommandLine *string
	Parameters                   []string
}

type LoginInfo struct {
	Username, Password *string
	Uid                int
}

type Subprocess struct {
	CurrentDirectory *string
	Environment      *[]string

	NoJob         bool
	RestrictUi    bool
	ProcessLimit  uint32
	CheckIdleness bool

	TimeLimit       uint64
	HardTimeLimit   uint64
	MemoryLimit     uint64
	HardMemoryLimit uint64
	TimeQuantum     uint32

	Cmd                   *CommandLine
	Login                 *LoginInfo
	StdIn, StdOut, StdErr *Redirect
}

func SubprocessCreate() *Subprocess {
	result := &Subprocess{}
	result.TimeQuantum = 1000

	return result
}
