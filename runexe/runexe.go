package main

import (
	"flag"
	"os"
	"fmt"
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

	LoginName string
	Password string
	InjectDLL string

	StdIn string
	StdOut string
	StdErr string

	ReturnExitCode bool
	Quiet bool
	StatsToFile string
	TrustedMode bool
	ShowKernelModeTime bool
	NoIdleCheck bool
	Xml bool
	XmlToFile string
}

func GetProcessConfig(args []string) *ProcessConfig {
	var result ProcessConfig
	fs := flag.NewFlagSet("subprocess", flag.PanicOnError)

	fs.Var(&result.TimeLimit, "t", "time limit")
	fs.Var(&result.MemoryLimit, "m", "memory limit")
	fs.StringVar(&result.CurrentDirectory, "d", "", "Current directory")
	fs.StringVar(&result.LoginName, "l", "", "Login name")
	fs.StringVar(&result.Password, "p", "", "Password")
	fs.StringVar(&result.InjectDLL, "j", "", "Inject DLL")
	fs.StringVar(&result.StdIn, "i", "", "StdIn")
	fs.StringVar(&result.StdOut, "o", "", "StdOut")
	fs.StringVar(&result.StdErr, "e", "", "StdErr")
	fs.BoolVar(&result.ReturnExitCode, "x", false, "Pass exit code")
	fs.BoolVar(&result.Quiet, "q", false, "Quiet")
	fs.StringVar(&result.StatsToFile, "s", "", "Store stats in file")
	// -D env var
	fs.BoolVar(&result.TrustedMode, "z", false, "trusted mode")
	fs.BoolVar(&result.ShowKernelModeTime, "show-kernel-mode-time", false, "Show kernel mode time")
	fs.BoolVar(&result.NoIdleCheck, "no-idleness-check", false, "no idle check")
	fs.BoolVar(&result.Xml, "xml", false, "Print xml")
	fs.StringVar(&result.XmlToFile, "xml-to-file", "", "xml to file")

	fs.Parse(args)

	result.ApplicationName = fs.Args()[0]
	result.CommandLine = strings.Join(fs.Args(), " ")

	return &result
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

func main() {
	s := GetProcessConfig(os.Args[1:])
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

	//sub.Environment = fillEnv(request.Environment)

	sub.StdIn = fillRedirect(s.StdIn)
	sub.StdOut = fillRedirect(s.StdOut)
	sub.StdErr = fillRedirect(s.StdErr)

	sub.Options = &subprocess.PlatformOptions{}

	globalData, err := platform.CreateGlobalData()
	if err != nil {
		l4g.Error(err)
		return
	}

	if s.NeedLogin() {
		sub.Login, err = subprocess.NewLoginInfo(s.LoginName, s.Password)
		if err != nil {
			l4g.Error(err)
			return
		}
		sub.Options.Desktop = globalData.DesktopName
	}

	result, err := sub.Execute()

	fmt.Println(result)
}
