package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/subprocess"

	log "github.com/sirupsen/logrus"
)

var version string
var buildid string

type ProcessConfig struct {
	ApplicationName  string
	CommandLine      string
	CurrentDirectory string
	Parameters       []string

	TimeLimit       TimeLimitFlag
	HardTimeLimit   TimeLimitFlag
	MemoryLimit     MemoryLimitFlag
	Environment     EnvFlag
	ProcessAffinity ProcessAffinityFlag

	LoginName string
	Password  string
	InjectDLL string

	StdIn         string
	StdOut        string
	StdErr        string
	JoinStdOutErr bool

	TrustedMode bool
	NoIdleCheck bool
	NoJob       bool
}

type RunexeConfig struct {
	Xml                 bool
	Interactor          string
	ShowKernelModeTime  bool
	ReturnExitCode      bool
	Logfile             string
	RecordProgramInput  string
	RecordProgramOutput string
}

type ProcessType int

const (
	PROGRAM    = ProcessType(0)
	INTERACTOR = ProcessType(1)
)

func (i ProcessType) String() string {
	switch i {
	case PROGRAM:
		return "Program"
	case INTERACTOR:
		return "Interactor"
	default:
		return "UNKNOWN"
	}
}

func CreateFlagSet() (*flag.FlagSet, *ProcessConfig) {
	var result ProcessConfig
	fs := flag.NewFlagSet("subprocess", flag.ExitOnError)
	fs.Usage = PrintUsage

	fs.Var(&result.TimeLimit, "t", "")
	fs.Var(&result.MemoryLimit, "m", "")
	fs.Var(&result.Environment, "D", "")
	fs.Var(&result.ProcessAffinity, "a", "")
	fs.Var(&result.HardTimeLimit, "h", "")
	fs.StringVar(&result.CurrentDirectory, "d", "", "")
	fs.StringVar(&result.LoginName, "l", "", "")
	fs.StringVar(&result.Password, "p", "", "")
	fs.StringVar(&result.InjectDLL, "j", "", "")
	fs.StringVar(&result.StdIn, "i", "", "")
	fs.StringVar(&result.StdOut, "o", "", "")
	fs.StringVar(&result.StdErr, "e", "", "")
	fs.BoolVar(&result.JoinStdOutErr, "u", false, "")
	fs.BoolVar(&result.TrustedMode, "z", false, "")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "")
	fs.BoolVar(&result.NoJob, "no-job", false, "")

	return fs, &result
}

func AddGlobalFlags(fs *flag.FlagSet) *RunexeConfig {
	var result RunexeConfig
	fs.BoolVar(&result.Xml, "xml", false, "")
	fs.StringVar(&result.Interactor, "interactor", "", "")
	fs.StringVar(&result.Logfile, "logfile", "", "")
	fs.StringVar(&result.RecordProgramInput, "ri", "", "")
	fs.StringVar(&result.RecordProgramOutput, "ro", "", "")
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "")
	fs.BoolVar(&result.ReturnExitCode, "x", false, "")
	return &result
}

func ParseFlagSet(fs *flag.FlagSet, pc *ProcessConfig, args []string) error {
	fs.Parse(args)

	if len(fs.Args()) < 1 {
		PrintUsage()
		os.Exit(2)
	}

	ArgsToPc(pc, fs.Args())
	return nil
}

func (pc *ProcessConfig) NeedLogin() bool {
	return pc.LoginName != "" && pc.Password != ""
}

func fillRedirect(x string) *subprocess.Redirect {
	if x == "" {
		return nil
	}
	return &subprocess.Redirect{
		Filename: x,
		Mode:     subprocess.REDIRECT_FILE,
	}
}

func SetupSubprocess(s *ProcessConfig, desktop *platform.ContesterDesktop, loadLibraryW uintptr) (*subprocess.Subprocess, error) {
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
	if s.HardTimeLimit > 0 {
		sub.HardTimeLimit = subprocess.DuFromMicros(uint64(s.HardTimeLimit))
	}
	sub.MemoryLimit = uint64(s.MemoryLimit)
	sub.CheckIdleness = !s.NoIdleCheck
	sub.RestrictUi = !s.TrustedMode
	sub.ProcessAffinityMask = uint64(s.ProcessAffinity)
	sub.NoJob = s.NoJob

	if len(s.Environment) > 0 {
		sub.Environment = s.Environment
		sub.NoInheritEnvironment = true
	}

	sub.StdIn = fillRedirect(s.StdIn)
	sub.StdOut = fillRedirect(s.StdOut)
	if s.JoinStdOutErr {
		sub.JoinStdOutErr = true
	} else {
		sub.StdErr = fillRedirect(s.StdErr)
	}

	sub.Options = newPlatformOptions()

	var err error
	if s.NeedLogin() {
		sub.Login, err = subprocess.NewLoginInfo(s.LoginName, s.Password)
		if err != nil {
			return nil, err
		}
		setDesktop(sub.Options, desktop)
	}

	setInject(sub.Options, s.InjectDLL, loadLibraryW)
	return sub, nil
}

func ExecAndSend(sub *subprocess.Subprocess, pr **RunResult, ptype ProcessType, wg *sync.WaitGroup) {
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
			r.V = CRASH
		} else {
			r.V = FAIL
		}
	} else {
		r.V = GetVerdict(r.R)
	}
	*pr = &r
}

func ParseFlags(globals bool, args []string) (pc *ProcessConfig, gc *RunexeConfig, err error) {
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
		Fail(globalFlags.Xml, err, "Parse main flags")
	}

	if globalFlags.Logfile != "" {
		logfile, err := os.Create(globalFlags.Logfile)
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(logfile)
	}

	var interactorFlags *ProcessConfig

	if globalFlags.Interactor != "" {
		interactorFlags, _, err = ParseFlags(false, strings.Split(globalFlags.Interactor, " "))
		if err != nil {
			Fail(globalFlags.Xml, err, "Parse interator flags")
		}
	}

	if globalFlags.Xml {
		fmt.Println(XML_HEADER)
	}

	desktop, err := CreateDesktopIfNeeded(programFlags, interactorFlags)
	if err != nil {
		Fail(globalFlags.Xml, err, "Create desktop if needed")
	}

	loadLibrary, err := GetLoadLibraryIfNeeded(programFlags, interactorFlags)
	if err != nil {
		Fail(globalFlags.Xml, err, "Load library if needed")
	}

	var program, interactor *subprocess.Subprocess
	program, err = SetupSubprocess(programFlags, desktop, loadLibrary)
	if err != nil {
		Fail(globalFlags.Xml, err, "Setup main subprocess")
	}

	var recorder subprocess.OrderedRecorder

	if interactorFlags != nil {
		interactor, err = SetupSubprocess(interactorFlags, desktop, loadLibrary)
		if err != nil {
			Fail(globalFlags.Xml, err, "Setup interactor subprocess")
		}

		var recordI, recordO *os.File

		if globalFlags.RecordProgramInput != "" {
			recordI, err = os.Create(globalFlags.RecordProgramInput)
			if err != nil {
				Fail(globalFlags.Xml, err, "Create input recorded")
			}
		}
		if globalFlags.RecordProgramOutput != "" {
			recordO, err = os.Create(globalFlags.RecordProgramOutput)
			if err != nil {
				Fail(globalFlags.Xml, err, "Create output recorder")
			}
		}

		err = subprocess.Interconnect(program, interactor, recordI, recordO, &recorder)
		if err != nil {
			Fail(globalFlags.Xml, err, "Interconnect")
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	var results [2]*RunResult
	if interactor != nil {
		wg.Add(1)
		go ExecAndSend(interactor, &results[1], INTERACTOR, &wg)
	}
	go ExecAndSend(program, &results[0], PROGRAM, &wg)
	wg.Wait()

	var programReturnCode int
	if results[0] != nil && results[0].R != nil {
		programReturnCode = int(results[0].R.ExitCode)
	}

	if globalFlags.Xml {
		fmt.Println(XML_RESULTS_START)
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		PrintResult(globalFlags.Xml, globalFlags.ShowKernelModeTime, result, recorder.GetEntries())
	}

	if globalFlags.Xml {
		fmt.Println(XML_RESULTS_END)
	}

	if globalFlags.ReturnExitCode {
		os.Exit(programReturnCode)
	}
}
