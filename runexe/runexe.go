package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"sync"

	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/subprocess"

	log "github.com/sirupsen/logrus"
)

var version string
var buildid string

type processConfig struct {
	ApplicationName  string
	CommandLine      string
	CurrentDirectory string
	Parameters       []string

	TimeLimit       timeLimitFlag
	WallTimeLimit   timeLimitFlag
	KernelTimeLimit timeLimitFlag
	MemoryLimit     memoryLimitFlag
	Environment     envFlag
	EnvironmentFile string
	ProcessAffinity processAffinityFlag

	LoginName string
	Password  string
	InjectDLL string

	StdIn         string
	StdOut        string
	StdErr        string
	JoinStdOutErr bool

	StdOutMaxSize int64
	StdErrMaxSize int64

	TrustedMode bool
	NoIdleCheck bool
	NoJob       bool

	ProcessLimit int
}

type runexeConfig struct {
	XML                  bool
	Interactor           string
	ShowKernelModeTime   bool
	ReturnExitCode       bool
	Logfile              string
	RecordProgramInput   string
	RecordProgramOutput  string
	RecordInteractionLog string
}

type processType int

const (
	processProgram    = processType(0)
	processInteractor = processType(1)
)

func (i processType) String() string {
	switch i {
	case processProgram:
		return "Program"
	case processInteractor:
		return "Interactor"
	default:
		return "UNKNOWN"
	}
}

func CreateFlagSet() (*flag.FlagSet, *processConfig) {
	var result processConfig
	fs := flag.NewFlagSet("subprocess", flag.ExitOnError)
	fs.Usage = printUsage

	fs.Var(&result.TimeLimit, "t", "")
	fs.Var(&result.MemoryLimit, "m", "")
	fs.Var(&result.Environment, "D", "")
	fs.Var(&result.ProcessAffinity, "a", "")
	fs.Var(&result.WallTimeLimit, "h", "")
	fs.StringVar(&result.CurrentDirectory, "d", "", "")
	fs.StringVar(&result.LoginName, "l", "", "")
	fs.StringVar(&result.Password, "p", "", "")
	fs.StringVar(&result.InjectDLL, "j", "", "")
	fs.StringVar(&result.StdIn, "i", "", "")
	fs.StringVar(&result.StdOut, "o", "", "")
	fs.Int64Var(&result.StdOutMaxSize, "os", 0, "")
	fs.Int64Var(&result.StdErrMaxSize, "es", 0, "")
	fs.StringVar(&result.StdErr, "e", "", "")
	fs.StringVar(&result.EnvironmentFile, "envfile", "", "")
	fs.BoolVar(&result.JoinStdOutErr, "u", false, "")
	fs.BoolVar(&result.TrustedMode, "z", false, "")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "")
	fs.BoolVar(&result.NoJob, "no-job", false, "")
	fs.IntVar(&result.ProcessLimit, "process-limit", 0, "")

	return fs, &result
}

func AddGlobalFlags(fs *flag.FlagSet) *runexeConfig {
	var result runexeConfig
	fs.BoolVar(&result.XML, "xml", false, "")
	fs.StringVar(&result.Interactor, "interactor", "", "")
	fs.StringVar(&result.Logfile, "logfile", "", "")
	fs.StringVar(&result.RecordProgramInput, "ri", "", "")
	fs.StringVar(&result.RecordProgramOutput, "ro", "", "")
	fs.StringVar(&result.RecordInteractionLog, "ilog", "", "")
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "")
	fs.BoolVar(&result.ReturnExitCode, "x", false, "")
	return &result
}

func ParseFlagSet(fs *flag.FlagSet, pc *processConfig, args []string) error {
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		printUsage()
		os.Exit(2)
	}

	argsToPc(pc, fs.Args())
	return nil
}

func (pc *processConfig) NeedLogin() bool {
	return pc.LoginName != "" && pc.Password != ""
}

func fillRedirect(x string, maxSize int64) *subprocess.Redirect {
	if x == "" {
		return nil
	}
	return &subprocess.Redirect{
		Filename:      x,
		Mode:          subprocess.REDIRECT_FILE,
		MaxOutputSize: maxSize,
	}
}

func readEnvironmentFile(name string) ([]string, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var result []string
	for scanner.Scan() {
		result = append(result, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func SetupSubprocess(s *processConfig, env *platform.GlobalData) (*subprocess.Subprocess, error) {
	sub := subprocess.SubprocessCreate()

	sub.Cmd = &subprocess.CommandLine{}

	if s.ApplicationName != "" {
		sub.Cmd.ApplicationName = s.ApplicationName
	}

	if s.CommandLine != "" {
		sub.Cmd.CommandLine = s.CommandLine
	}

	if s.Parameters != nil {
		sub.Cmd.Parameters = s.Parameters
	}

	if s.CurrentDirectory != "" {
		sub.CurrentDirectory = s.CurrentDirectory
	} else {
		if wd, err := os.Getwd(); err == nil && wd != "" {
			sub.CurrentDirectory = wd
		}
	}

	sub.TimeLimit = subprocess.DuFromMicros(uint64(s.TimeLimit))
	if s.WallTimeLimit > 0 {
		sub.WallTimeLimit = subprocess.DuFromMicros(uint64(s.WallTimeLimit))
	}
	sub.MemoryLimit = uint64(s.MemoryLimit)
	sub.CheckIdleness = !s.NoIdleCheck
	sub.RestrictUi = !s.TrustedMode
	sub.ProcessAffinityMask = uint64(s.ProcessAffinity)
	sub.NoJob = s.NoJob
	if s.ProcessLimit > 0 {
		if s.NoJob {
			return nil, errors.New("can't enforce process limit if not using job object")
		}
		sub.FailOnJobCreationFailure = true
		sub.ProcessLimit = uint32(s.ProcessLimit)
	}

	if s.EnvironmentFile != "" {
		var err error
		if sub.Environment, err = readEnvironmentFile(s.EnvironmentFile); err != nil {
			return nil, err
		}
		sub.NoInheritEnvironment = true
	} else if len(s.Environment) > 0 {
		sub.Environment = s.Environment
		sub.NoInheritEnvironment = true
	}

	sub.StdIn = fillRedirect(s.StdIn, 0)
	sub.StdOut = fillRedirect(s.StdOut, s.StdOutMaxSize)
	if s.JoinStdOutErr {
		sub.JoinStdOutErr = true
	} else {
		sub.StdErr = fillRedirect(s.StdErr, s.StdErrMaxSize)
	}

	sub.Options = newPlatformOptions()
	sub.Options.Environment = env

	var err error
	if s.NeedLogin() {
		sub.Login, err = subprocess.NewLoginInfo(s.LoginName, s.Password)
		if err != nil {
			return nil, err
		}
	}

	setInject(sub.Options, s.InjectDLL)
	return sub, nil
}

func ExecAndSend(sub *subprocess.Subprocess, pr **RunResult, ptype processType, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	r := RunResult{
		T: ptype,
		S: sub,
	}
	r.R, r.E = sub.Execute()
	if r.E != nil {
		if subprocess.IsUserError(r.E) {
			r.V = verdictCrash
		} else {
			r.V = verdictFail
		}
	} else {
		r.V = getVerdict(r.R)
	}
	*pr = &r
}

func ParseFlags(globals bool, args []string) (pc *processConfig, gc *runexeConfig, err error) {
	var fs *flag.FlagSet

	fs, pc = CreateFlagSet()
	if globals {
		gc = AddGlobalFlags(fs)
	}
	ParseFlagSet(fs, pc, args)
	return
}

func main() {
	programFlags, globalFlags, err := ParseFlags(true, os.Args[1:])
	if err != nil {
		Fail(err, "Parse main flags")
	}

	if globalFlags.Logfile != "" {
		logfile, err := os.Create(globalFlags.Logfile)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(logfile)
	}

	var interactorFlags *processConfig

	if globalFlags.Interactor != "" {
		interactorFlags, _, err = ParseFlags(false, commandLineToArgv(globalFlags.Interactor))
		if err != nil {
			Fail(err, "Parse interactor flags")
		}
	}

	if globalFlags.XML {
		fmt.Println(xmlHeaderText)
		failLog = FailXml
	}

	globalData, err := platform.CreateGlobalData(desktopNeeded(programFlags, interactorFlags))

	if err != nil {
		Fail(err, "Creating platform data")
	}

	// This is a temporary hack to always create and run 32-bit detector.
	// If we run it concurrently to our custom createprocess, it fails somewhere
	// in go runtime.
	globalData.GetLoadLibraryW32()

	var program, interactor *subprocess.Subprocess
	program, err = SetupSubprocess(programFlags, globalData)
	if err != nil {
		Fail(err, "Setup main subprocess")
	}

	var recorder *subprocess.OrderedRecorder = nil
	if !globalFlags.XML {
		recorder = &subprocess.OrderedRecorder{}
	}

	if interactorFlags != nil {
		interactor, err = SetupSubprocess(interactorFlags, globalData)
		if err != nil {
			Fail(err, "Setup interactor subprocess")
		}

		var recordI, recordO, recordInteractionLog *os.File

		if globalFlags.RecordProgramInput != "" {
			recordI, err = os.Create(globalFlags.RecordProgramInput)
			if err != nil {
				Fail(err, "Create input recorded")
			}
		}
		if globalFlags.RecordProgramOutput != "" {
			recordO, err = os.Create(globalFlags.RecordProgramOutput)
			if err != nil {
				Fail(err, "Create output recorder")
			}
		}
		if globalFlags.RecordInteractionLog != "" {
			recordInteractionLog, err = os.Create(globalFlags.RecordInteractionLog)
			if err != nil {
				Fail(err, "Create interaction log recorder")
			}
		}

		var pipeResultRecorder subprocess.PipeResultRecorder = nil
		if recorder != nil {
			pipeResultRecorder = recorder
		}
		err = subprocess.Interconnect(
			program, interactor,
			recordI, recordO, recordInteractionLog,
			pipeResultRecorder)
		if err != nil {
			Fail(err, "Interconnect")
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var results [2]*RunResult
	if interactor != nil {
		wg.Add(1)
		go ExecAndSend(interactor, &results[1], processInteractor, &wg)
	}
	go ExecAndSend(program, &results[0], processProgram, &wg)
	wg.Wait()

	var programReturnCode int
	if results[0] != nil && results[0].R != nil {
		programReturnCode = int(results[0].R.ExitCode)
	}

	if globalFlags.XML {
		PrintResultsXml(results[:])
	} else {
		for _, result := range results {
			if result == nil {
				continue
			}
			PrintResultText(globalFlags.ShowKernelModeTime, result, recorder.GetEntries())
		}
	}

	if globalFlags.ReturnExitCode {
		os.Exit(programReturnCode)
	}
}

// appendBSBytes appends n '\\' bytes to b and returns the resulting slice.
func appendBSBytes(b []byte, n int) []byte {
	for ; n > 0; n-- {
		b = append(b, '\\')
	}
	return b
}

// readNextArg splits command line string cmd into next
// argument and command line remainder.
func readNextArg(cmd string) (arg []byte, rest string) {
	var b []byte
	var inquote bool
	var nslash int
	for ; len(cmd) > 0; cmd = cmd[1:] {
		c := cmd[0]
		switch c {
		case ' ', '\t':
			if !inquote {
				return appendBSBytes(b, nslash), cmd[1:]
			}
		case '"':
			b = appendBSBytes(b, nslash/2)
			if nslash%2 == 0 {
				// use "Prior to 2008" rule from
				// http://daviddeley.com/autohotkey/parameters/parameters.htm
				// section 5.2 to deal with double double quotes
				if inquote && len(cmd) > 1 && cmd[1] == '"' {
					b = append(b, c)
					cmd = cmd[1:]
				}
				inquote = !inquote
			} else {
				b = append(b, c)
			}
			nslash = 0
			continue
		case '\\':
			nslash++
			continue
		}
		b = appendBSBytes(b, nslash)
		nslash = 0
		b = append(b, c)
	}
	return appendBSBytes(b, nslash), ""
}

// commandLineToArgv splits a command line into individual argument
// strings, following the Windows conventions documented
// at http://daviddeley.com/autohotkey/parameters/parameters.htm#WINARGV
func commandLineToArgv(cmd string) []string {
	var args []string
	for len(cmd) > 0 {
		if cmd[0] == ' ' || cmd[0] == '\t' {
			cmd = cmd[1:]
			continue
		}
		var arg []byte
		arg, cmd = readNextArg(cmd)
		args = append(args, string(arg))
	}
	return args
}
