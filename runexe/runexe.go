package main

import (
	"flag"
	"os"
	"fmt"
	"strings"
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

func main() {
	s := GetProcessConfig(os.Args[1:])
	fmt.Println(s)
}
