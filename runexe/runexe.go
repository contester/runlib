package main

import (
	"fmt"
	"flag"
	"os"
	"strings"
	l4g "code.google.com/p/log4go"

	"runlib/subprocess"
"runlib/platform"
	"io"
	"bytes"
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
	Quiet bool
	Xml bool
	Interactor string
	XmlToFile string
	StatsToFile string
	ShowKernelModeTime bool
	ReturnExitCode bool
	Logfile string
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
	fs.BoolVar(&result.Quiet, "q", false, "Quiet")
	fs.BoolVar(&result.Xml, "xml", false, "Print xml")
	fs.StringVar(&result.Interactor, "interactor", "", "Interactor")
	fs.StringVar(&result.XmlToFile, "xml-to-file", "", "xml to file")
	fs.StringVar(&result.StatsToFile, "s", "", "Store stats in file")
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

type InteractorPipes struct {
	read1, read2, write1, write2 *os.File
}

func SetupSubprocess(s *ProcessConfig, desktop *platform.ContesterDesktop, loadLibraryW uintptr, pipes *InteractorPipes, isInteractor bool) (*subprocess.Subprocess, error) {
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

	if pipes != nil {
		sub.StdIn = &subprocess.Redirect{
			Mode: subprocess.REDIRECT_PIPE,
		}
		sub.StdOut = &subprocess.Redirect{
			Mode: subprocess.REDIRECT_PIPE,
		}
		if isInteractor {
			sub.StdIn.Pipe = pipes.read1
			sub.StdOut.Pipe = pipes.write2
		} else {
			sub.StdIn.Pipe = pipes.read2
			sub.StdOut.Pipe = pipes.write1
		}
	} else {
		sub.StdIn = fillRedirect(s.StdIn)
		sub.StdOut = fillRedirect(s.StdOut)
	}
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

func CreatePipes() *InteractorPipes {
	var result InteractorPipes
	var err error
	result.read1, result.write1, err = os.Pipe()
	if err != nil {
		return nil
	}
	result.read2, result.write2, err = os.Pipe()
	if err != nil {
		return nil
	}
	return &result
}

type resultAndError struct {
	s *subprocess.Subprocess
	r *subprocess.SubprocessResult
	e error
	isInteractor bool
}

func ExecAndSend(sub *subprocess.Subprocess, c chan resultAndError, isInteractor bool) {
	var r resultAndError
	r.isInteractor = isInteractor
	r.s = sub
	r.r, r.e = sub.Execute()
	c <- r
}

func main() {
	fs, s := CreateFlagSet()
	gc := AddGlobalFlags(fs)
	ParseFlagSet(fs, s, os.Args[1:])
	if gc.Logfile != "" {
		l4g.Global.AddFilter("log", l4g.FINE, l4g.NewFileLogWriter(gc.Logfile, true))
	}

	var out io.Writer
	var err error

	switch {
	case gc.Quiet:
		out = &bytes.Buffer{}
	case gc.XmlToFile != "":
		gc.Xml = true
		out, err = os.Create(gc.XmlToFile)
	case gc.StatsToFile != "":
		out, err = os.Create(gc.StatsToFile)
	}

	if out == nil {
		out = os.Stdout
	}

	if gc.Xml {
		fmt.Fprintln(out, XML_HEADER)
	}

	needDesktop := s.NeedLogin()
	needLoadLibrary := s.InjectDLL != ""

	var pipes *InteractorPipes
	var isub *subprocess.Subprocess
	var i *ProcessConfig

	if gc.Interactor != "" {
		pipes = CreatePipes()
		var is *flag.FlagSet
		is, i = CreateFlagSet()
		ParseFlagSet(is, i, strings.Split(gc.Interactor, " "))
		needDesktop = needDesktop && i.NeedLogin()
		needLoadLibrary = needLoadLibrary && s.InjectDLL != ""
	}

	var desktop *platform.ContesterDesktop
	if needDesktop {
		desktop, err = platform.CreateContesterDesktopStruct()
		if err != nil {
			Crash(out, "can't create winsta/desktop", err)
		}
	}

	var loadLibrary uintptr
	if needLoadLibrary {
		loadLibrary, err = platform.GetLoadLibrary()
		if err != nil {
			Crash(out, "can't get LoadLibraryW address", err)
		}
	}

	if gc.Interactor != "" {
		isub, err = SetupSubprocess(i, desktop, loadLibrary, pipes, true)
		if err != nil {
			Crash(out, "can't setup interactor", err)
		}

	}

	sub, err := SetupSubprocess(s, desktop, loadLibrary, pipes, false)
	if err != nil {
		Crash(out, "can't setup process", err)
	}

	cs := make(chan resultAndError, 1)

	j := 1

	if isub != nil {
		j++
		go ExecAndSend(isub, cs, true)
	}
	go ExecAndSend(sub, cs, false)


	var exitCode int

	for j > 0 {
		r := <- cs
		j--
		var c string
		if r.isInteractor {
			c = "Interactor"
		} else {
			c = "Process"
		}
		if r.e != nil {
			Crash(out, "can't execute '" + *r.s.Cmd.CommandLine + "'", r.e)
		} else {
			if !r.isInteractor && gc.ReturnExitCode {
				exitCode = int(r.r.ExitCode)
			}
			if gc.Xml {
				out.Write(XmlResult(r.r, c)[:])
			} else {
				PrintResult(out, r.s, r.r, c, gc.ShowKernelModeTime)
			}
		}
	}

	os.Exit(exitCode)
}
