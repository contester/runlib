package main

import (
	"fmt"
	"flag"
	"os"
	"strings"
	l4g "code.google.com/p/log4go"

	"runlib/subprocess"
"runlib/platform"
)

type ProcessConfig struct {
	ApplicationName string
	CommandLine string
	CurrentDirectory string

	TimeLimit TimeLimitFlag
	MemoryLimit MemoryLimitFlag
	Environment EnvFlag

	LoginName string
	Password string
	InjectDLL string

	StdIn string
	StdOut string
	StdErr string

	TrustedMode bool
	NoIdleCheck bool
}

type RunexeConfig struct {
	Xml bool
	Interactor string
	ShowKernelModeTime bool
	ReturnExitCode bool
	Logfile string
}

type ProcessType int

const (
	PROGRAM = ProcessType(0)
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
	fs := flag.NewFlagSet("subprocess", flag.PanicOnError)

	fs.Var(&result.TimeLimit, "t", "time limit")
	fs.Var(&result.MemoryLimit, "m", "memory limit")
	fs.Var(&result.Environment, "D", "environment")
	fs.StringVar(&result.CurrentDirectory, "d", "", "Current directory")
	fs.StringVar(&result.LoginName, "l", "", "Login name")
	fs.StringVar(&result.Password, "p", "", "Password")
	fs.StringVar(&result.InjectDLL, "j", "", "Inject DLL")
	fs.StringVar(&result.StdIn, "i", "", "StdIn")
	fs.StringVar(&result.StdOut, "o", "", "StdOut")
	fs.StringVar(&result.StdErr, "e", "", "StdErr")
	fs.BoolVar(&result.TrustedMode, "z", false, "trusted mode")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "no idle check")

	return fs, &result
}

func AddGlobalFlags(fs *flag.FlagSet) *RunexeConfig {
	var result RunexeConfig
	fs.BoolVar(&result.Xml, "xml", false, "Print xml")
	fs.StringVar(&result.Interactor, "interactor", "", "Interactor")
	fs.StringVar(&result.Logfile, "logfile", "", "Logfile")
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "Show kernel mode time")
	fs.BoolVar(&result.ReturnExitCode, "x", false, "Pass exit code")
	return &result
}

func ParseFlagSet(fs *flag.FlagSet, pc *ProcessConfig, args []string) error {
	fs.Parse(args)

	// pc.ApplicationName = fs.Args()[0]
	pc.CommandLine = strings.Join(fs.Args(), " ")

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
		Mode: subprocess.REDIRECT_FILE,
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

	if s.CurrentDirectory != "" {
		sub.CurrentDirectory = &s.CurrentDirectory
	}

	sub.TimeLimit = uint64(s.TimeLimit)
	sub.HardTimeLimit = sub.TimeLimit * 10
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
		if desktop != nil {
			sub.Options.Desktop = desktop.DesktopName
		}
	}

	if s.InjectDLL != "" && loadLibraryW != 0 {
		sub.Options.InjectDLL = s.InjectDLL
		sub.Options.LoadLibraryW = loadLibraryW
	}
	return sub, nil
}

func ExecAndSend(sub *subprocess.Subprocess, c chan RunResult, ptype ProcessType) {
	var r RunResult
	r.T = ptype
	r.S = sub
	r.R, r.E = sub.Execute()
	if r.E != nil {
		r.V = CRASH
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

func CreateDesktopIfNeeded(program, interactor *ProcessConfig) (*platform.ContesterDesktop, error) {
	if !program.NeedLogin() && (interactor != nil && !interactor.NeedLogin()) {
		return nil, nil
	}

	return platform.CreateContesterDesktopStruct()
}

func GetLoadLibraryIfNeeded(program, interactor *ProcessConfig) (uintptr, error) {
	if program.InjectDLL == "" && (interactor == nil || interactor.InjectDLL == "") {
		return 0, nil
	}
	return platform.GetLoadLibrary()
}

func main() {
	l4g.Global = l4g.Logger{}

	programFlags, globalFlags, err := ParseFlags(true, os.Args[1:])

	if globalFlags.Logfile != "" {
		l4g.Global.AddFilter("log", l4g.FINE, l4g.NewFileLogWriter(globalFlags.Logfile, true))
	}

	var interactorFlags *ProcessConfig

	if globalFlags.Interactor != "" {
		interactorFlags, _, err = ParseFlags(false, strings.Split(globalFlags.Interactor, " "))
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

	if interactorFlags != nil {
		interactor, err = SetupSubprocess(interactorFlags, desktop, loadLibrary)
		if err != nil {
			Fail(globalFlags.Xml, err)
		}
		err = subprocess.Interconnect(program, interactor)
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

	for outstanding > 0 {
		r := <- cs
		outstanding--
		results[int(r.T)] = &r
	}

	for _, result := range results {
		if result == nil {
			continue
		}

		PrintResult(globalFlags.Xml, globalFlags.ShowKernelModeTime, result)
	}
}
