package main

import (
	l4g "code.google.com/p/log4go"
	"flag"
	"fmt"
	"os"
	"strings"

	"runlib/platform"
	"runlib/subprocess"
)

var version string
var buildid string

type ProcessConfig struct {
	ApplicationName  string
	CommandLine      string
	CurrentDirectory string
	Parameters []string

	TimeLimit   TimeLimitFlag
	MemoryLimit MemoryLimitFlag
	Environment EnvFlag

	LoginName string
	Password  string
	InjectDLL string

	StdIn  string
	StdOut string
	StdErr string

	TrustedMode bool
	NoIdleCheck bool
}

type RunexeConfig struct {
	Xml                bool
	Interactor         string
	ShowKernelModeTime bool
	ReturnExitCode     bool
	Logfile            string
	RecordProgramInput string
	RecordProgramOutput string
}

type ProcessType int

const (
	PROGRAM    = ProcessType(0)
	INTERACTOR = ProcessType(1)
)

func (i *ProcessType) String() string {
	if i == nil {
		return "UNKNOWN"
	}
	switch *i {
	case PROGRAM:
		return "Program"
	case INTERACTOR:
		return "Interactor"
	}
	return "UNKNOWN"
}

func CreateFlagSet() (*flag.FlagSet, *ProcessConfig) {
	var result ProcessConfig
	fs := flag.NewFlagSet("subprocess", flag.ExitOnError)
	fs.Usage = PrintUsage

	fs.Var(&result.TimeLimit, "t", "")
	fs.Var(&result.MemoryLimit, "m", "")
	fs.Var(&result.Environment, "D", "")
	fs.StringVar(&result.CurrentDirectory, "d", "", "")
	fs.StringVar(&result.LoginName, "l", "", "")
	fs.StringVar(&result.Password, "p", "", "")
	fs.StringVar(&result.InjectDLL, "j", "", "")
	fs.StringVar(&result.StdIn, "i", "", "")
	fs.StringVar(&result.StdOut, "o", "", "")
	fs.StringVar(&result.StdErr, "e", "", "")
	fs.BoolVar(&result.TrustedMode, "z", false, "")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "")

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
		Filename: &x,
		Mode:     subprocess.REDIRECT_FILE,
	}
}

func SetupSubprocess(s *ProcessConfig, desktop *platform.ContesterDesktop, loadLibraryW uintptr) (*subprocess.Subprocess, error) {
	sub := subprocess.SubprocessCreate()

	sub.Cmd = &subprocess.CommandLine{}

	if s.ApplicationName != "" {
		sub.Cmd.ApplicationName = &s.ApplicationName
	}

	if s.CommandLine != "" {
		sub.Cmd.CommandLine = &s.CommandLine
	}

	if s.Parameters != nil {
		sub.Cmd.Parameters = s.Parameters
	}

	if s.CurrentDirectory != "" {
		sub.CurrentDirectory = &s.CurrentDirectory
	}

	sub.TimeLimit = uint64(s.TimeLimit)
	//sub.HardTimeLimit = sub.TimeLimit * 10
	sub.MemoryLimit = uint64(s.MemoryLimit)
	sub.CheckIdleness = !s.NoIdleCheck
	sub.RestrictUi = !s.TrustedMode

	if len(s.Environment) > 0 {
		sub.Environment = (*[]string)(&s.Environment)
	}

	sub.StdIn = fillRedirect(s.StdIn)
	sub.StdOut = fillRedirect(s.StdOut)
	sub.StdErr = fillRedirect(s.StdErr)

	sub.Options = &subprocess.PlatformOptions{}

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

func ExecAndSend(sub *subprocess.Subprocess, c chan RunResult, ptype ProcessType) {
	var r RunResult
	r.T = ptype
	r.S = sub
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
	c <- r
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
	l4g.Global = l4g.Logger{}

	programFlags, globalFlags, err := ParseFlags(true, os.Args[1:])

	if err != nil {
		Fail(globalFlags.Xml, err)
	}

	if globalFlags.Logfile != "" {
		l4g.Global.AddFilter("log", l4g.FINE, l4g.NewFileLogWriter(globalFlags.Logfile, true))
	}

	var interactorFlags *ProcessConfig

	if globalFlags.Interactor != "" {
		interactorFlags, _, err = ParseFlags(false, strings.Split(globalFlags.Interactor, " "))
		if err != nil {
			Fail(globalFlags.Xml, err)
		}
	}

	if globalFlags.Xml {
		fmt.Println(XML_HEADER)
	}

	desktop, err := CreateDesktopIfNeeded(programFlags, interactorFlags)
	if err != nil {
		Fail(globalFlags.Xml, err)
	}

	loadLibrary, err := GetLoadLibraryIfNeeded(programFlags, interactorFlags)
	if err != nil {
		Fail(globalFlags.Xml, err)
	}

	var program, interactor *subprocess.Subprocess
	program, err = SetupSubprocess(programFlags, desktop, loadLibrary)
	if err != nil {
		Fail(globalFlags.Xml, err)
	}

	var recordI, recordO *os.File

	if interactorFlags != nil {
		interactor, err = SetupSubprocess(interactorFlags, desktop, loadLibrary)
		if err != nil {
			Fail(globalFlags.Xml, err)
		}

		if globalFlags.RecordProgramInput != "" {
			recordI, err = os.Create(globalFlags.RecordProgramInput)
			if err != nil {
				Fail(globalFlags.Xml, err)
			}
		}
		if globalFlags.RecordProgramOutput != "" {
			recordO, err = os.Create(globalFlags.RecordProgramOutput)
			if err != nil {
				Fail(globalFlags.Xml, err)
			}
		}

		err = subprocess.Interconnect(program, interactor, recordI, recordO)
		if err != nil {
			Fail(globalFlags.Xml, err)
		}
	}

	cs := make(chan RunResult, 1)
	outstanding := 1
	if interactor != nil {
		outstanding++
		go ExecAndSend(interactor, cs, INTERACTOR)
	}
	go ExecAndSend(program, cs, PROGRAM)

	var results [2]*RunResult

	var programReturnCode int

	for outstanding > 0 {
		r := <-cs
		outstanding--
		results[int(r.T)] = &r
		if r.T == PROGRAM && r.R != nil {
		    programReturnCode = int(r.R.ExitCode)
		}
	}

	if globalFlags.Xml {
		fmt.Println(XML_RESULTS_START)
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		PrintResult(globalFlags.Xml, globalFlags.ShowKernelModeTime, result)
	}

	if globalFlags.Xml {
		fmt.Println(XML_RESULTS_END)
	}

	if globalFlags.ReturnExitCode {
		os.Exit(programReturnCode)
	}
}
