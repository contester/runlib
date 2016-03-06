package main

import (
	"strings"

	"github.com/contester/runlib/linux"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/subprocess"
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
	var opts subprocess.PlatformOptions
	var err error
	if opts.Cg, err = linux.NewCgroups(); err != nil {
		return nil
	}
	return &opts
}

func ArgsToPc(pc *ProcessConfig, args []string) {
	pc.ApplicationName = args[0]
	pc.CommandLine = strings.Join(args, " ")
	pc.Parameters = args
}
