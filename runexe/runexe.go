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
	"runlib/platform/win32"
	"syscall"
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

	ReturnExitCode bool
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
	fs.BoolVar(&result.ReturnExitCode, "x", false, "Pass exit code")
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
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "Show kernel mode time")
	return &result
}

func ParseFlagSet(fs *flag.FlagSet, pc *ProcessConfig, args []string) error {
	fs.Parse(args)

	pc.ApplicationName = fs.Args()[0]
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

func SetupSubprocess(s *ProcessConfig, g *platform.GlobalData, pipes *InteractorPipes, isInteractor bool) *subprocess.Subprocess {
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
			l4g.Error(err)
			return nil
		}
		sub.Options.Desktop = g.DesktopName
	}

	if s.InjectDLL != "" {
		sub.Options.InjectDLL = s.InjectDLL
		sub.Options.LoadLibraryW = g.LoadLibraryW
	}
	return sub
}

func CreatePipes() *InteractorPipes {
	var result InteractorPipes
	var err error
	result.read1, result.write1, err = os.Pipe()
	if err != nil {
		return nil
	}
	err = win32.SetInheritHandle(syscall.Handle(result.read1.Fd()), true)
	if err != nil {
		return nil
	}
	err = win32.SetInheritHandle(syscall.Handle(result.write1.Fd()), true)
	if err != nil {
		return nil
	}
	result.read2, result.write2, err = os.Pipe()
	if err != nil {
		return nil
	}
	err = win32.SetInheritHandle(syscall.Handle(result.read2.Fd()), true)
	if err != nil {
		return nil
	}
	err = win32.SetInheritHandle(syscall.Handle(result.write2.Fd()), true)
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

	globalData, err := platform.CreateGlobalData()
	if err != nil {
		l4g.Error(err)
		return
	}

	var pipes *InteractorPipes
	var isub *subprocess.Subprocess

	if gc.Interactor != "" {
		pipes = CreatePipes()
		is, i := CreateFlagSet()
		ParseFlagSet(is, i, strings.Split(gc.Interactor, " "))
		isub = SetupSubprocess(i, globalData, pipes, true)
	}

	sub := SetupSubprocess(s, globalData, pipes, false)

	cs := make(chan resultAndError, 1)

	i := 1

	if isub != nil {
		i++
		go ExecAndSend(isub, cs, true)
	}
	go ExecAndSend(sub, cs, false)

	var out, xmlOut io.Writer

	switch {
	case gc.Quiet:
		out = &bytes.Buffer{}
	case gc.XmlToFile != "":
		gc.Xml = true
		xmlOut, err = os.Create(gc.XmlToFile)
	case gc.StatsToFile != "":
		out, err = os.Create(gc.StatsToFile)
	}

	if out == nil {
		out = os.Stdout
	}

	if xmlOut == nil {
		xmlOut = os.Stdout
	}

	if gc.Xml {
		fmt.Fprintln(xmlOut, XML_HEADER)
	}

	for i > 0 {
		r := <- cs
		i--
		var c string
		if r.isInteractor {
			c = "Interactor"
		} else {
			c = "Process"
		}
		if r.e != nil {
			l4g.Error(c, r.e)
		} else {
			PrintResult(out, r.s, r.r, c, gc.ShowKernelModeTime)
			if gc.Xml {
				xmlOut.Write(XmlResult(r.r, c)[:])
			}
		}
	}
}
