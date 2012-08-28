package main

import (
	"flag"
	"os"
	"fmt"
	"runlib/subprocess"
	"strings"
)

func ConfigureSubprocess(args []string) error {
	result := subprocess.SubprocessCreate()
	fs := flag.NewFlagSet("subprocess", flag.PanicOnError)
	var timeLimit TimeLimit
	var memoryLimit MemoryLimit
	var err error

	fs.Var(&timeLimit, "t", "time limit")
	fs.Var(&memoryLimit, "m", "memory limit")
	currentDirectory := fs.String("d", "", "Current directory")
	loginName := fs.String("l", "", "Login name")
	password := fs.String("p", "", "Password")
	injectDll := fs.String("j", "", "Inject DLL")
	stdIn := fs.String("i", "", "StdIn")
	stdOut := fs.String("o", "", "StdOut")
	stdErr := fs.String("e", "", "StdErr")
	passExitCode := fs.Bool("x", false, "Pass exit code")
	quiet := fs.Bool("q", false, "Quiet")
	statsToFile := fs.String("s", "", "Store stats in file")
	// -D env var
	trustedMode := fs.Bool("z", false, "trusted mode")
	showKernelModeTime := fs.Bool("show-kernel-mode-time", false, "Show kernel mode time")
	noIdleCheck := fs.Bool("no-idleness-check", false, "no idle check")
	xml := fs.Bool("xml", false, "Print xml")
	xmlToFile := fs.String("xml-to-file", "", "xml to file")

	fs.Parse(args)

	result.MemoryLimit = memoryLimit
	result.TimeLimit = timeLimit
	result.Cmd = &subprocess.CommandLine{
		ApplicationName: fs.Args()[0],
		CommandLine: strings.Join(fs.Args(), " "),
		Parameters: fs.Args()[1:],
	}
	result.CurrentDirectory = currentDirectory
	result.CheckIdleness = !*noIdleCheck
	result.Options = &subprocess.PlatformOptions{}
	if *loginName != "" && *password != "" {
		result.Login, err = subprocess.NewLoginInfo(*loginName, *password)
	}
	if !*trustedMode {
		result.ProcessLimit = 1
		result.RestrictUi = 1
	}
	if *injectDll != "" {
	}
}

func main() {
	s := ConfigureSubprocess(os.Args[1:])
}
