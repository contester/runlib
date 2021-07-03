package main

import (
	"strings"

	"github.com/contester/runlib/linux"
	"github.com/contester/runlib/subprocess"
)

func desktopNeeded(program, interactor *processConfig) bool {
	return false
}

func loadLibraryNeeded(program, interactor *processConfig) bool {
	return false
}

func setInject(p *subprocess.PlatformOptions, injectDll string) {
}

func newPlatformOptions() *subprocess.PlatformOptions {
	var opts subprocess.PlatformOptions
	var err error
	if opts.Cg, err = linux.NewCgroups(); err != nil {
		return nil
	}
	return &opts
}

func argsToPc(pc *processConfig, args []string) {
	pc.ApplicationName = args[0]
	pc.CommandLine = strings.Join(args, " ")
	pc.Parameters = args
}
