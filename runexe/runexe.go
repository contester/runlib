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

type ProcessConfig struct {
	ApplicationName  string
	CommandLine      string
	CurrentDirectory string

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
	fs := flag.NewFlagSet("subprocess", flag.PanicOnError)

	fs.Var(&result.TimeLimit, "t", "time limit, terminate after <time-limit> seconds, you can\n"+
		"   add \"ms\" (without quotes) after the number to specify\n"+
		"   time limit in milliseconds.")
	fs.Var(&result.MemoryLimit, "m", "memory limit, terminate if anonymous virtual memory of the process\n"+
		"   exceeds <mem-limit> bytes, you can add K or M to specify\n"+
		"   memory limit in kilo- or megabytes")
	fs.Var(&result.Environment, "D", "environment")
	fs.StringVar(&result.CurrentDirectory, "d", "", "make <directory> home directory for process")
	fs.StringVar(&result.LoginName, "l", "", "create process under <login-name>")
	fs.StringVar(&result.Password, "p", "", "logins user using <password>")
	fs.StringVar(&result.InjectDLL, "j", "", "injects specified dll file into the process")
	fs.StringVar(&result.StdIn, "i", "", "redirects standart input stream to the <file>")
	fs.StringVar(&result.StdOut, "o", "", "redirects standart output stream to the <file>")
	fs.StringVar(&result.StdErr, "e", "", "redirects standart error stream to the <file>")
	fs.BoolVar(&result.TrustedMode, "z", false, "run process in trusted mode")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "switch off idleness checking")

	return fs, &result
}

func AddGlobalFlags(fs *flag.FlagSet) *RunexeConfig {
	var result RunexeConfig
	fs.BoolVar(&result.Xml, "xml", false, "form xml document with invocation result information")
	fs.StringVar(&result.Interactor, "interactor", "", "INTERACTOR MODE.\n"+
		"   Launch another process and cross-connect its stdin&stdout with the main program.\n"+
		"   Inside this flag, you can specify any process-controlling flags: interactor can have its\n"+
		"   own limits, credentials, environment, directory. In interactor mode, however, -i and -o\n"+
		"   have no effects on both main program and interactor.")
	fs.StringVar(&result.Logfile, "logfile", "", "Dump extra error info in logfile (DEPRECATED)")
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "show user-mode and kernel-mode time")
	fs.BoolVar(&result.ReturnExitCode, "x", false, "return exit code of the application (NOT IMPLEMENTED)")
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
		r := <-cs
		outstanding--
		results[int(r.T)] = &r
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
}
