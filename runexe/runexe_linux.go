package main

import (
	"runlib/linux"
	"runlib/platform"
	"runlib/subprocess"
	"strings"
)

func CreateDesktopIfNeeded(program, interactor *ProcessConfig) (*platform.ContesterDesktop, error) {
	return nil, nil
}

func GetLoadLibraryIfNeeded(program, interactor *ProcessConfig) (uintptr, error) {
	return 0, nil
}

func setDesktop(p *subprocess.PlatformOptions, desktop *platform.ContesterDesktop) {
}

func setInject(p *subprocess.PlatformOptions, injectDll string, loadLibraryW uintptr) {
}

func newPlatformOptions() *subprocess.PlatformOptions {
	return &subprocess.PlatformOptions{
		Cg: linux.NewCgroups(),
	}
}

func ArgsToPc(pc *ProcessConfig, args []string) {
	pc.ApplicationName = args[0]
	pc.CommandLine = strings.Join(args, " ")
	pc.Parameters = args
}
