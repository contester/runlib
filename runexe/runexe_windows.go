package main

import (
	"strings"

	"github.com/contester/runlib/subprocess"
)

func desktopNeeded(program, interactor *processConfig) bool {
	if !program.NeedLogin() {
		if interactor == nil || !interactor.NeedLogin() {
			return false
		}
	}

	return true
}

func loadLibraryNeeded(program, interactor *processConfig) bool {
	if program.InjectDLL == "" && (interactor == nil || interactor.InjectDLL == "") {
		return false
	}
	return true
}

func setInject(p *subprocess.PlatformOptions, injectDll string) {
	if injectDll != "" {
		p.InjectDLL = []string{injectDll}
	}
}

func newPlatformOptions() *subprocess.PlatformOptions {
	return &subprocess.PlatformOptions{}
}

func argsToPc(pc *processConfig, args []string) {
	pc.CommandLine = strings.Join(args, " ")
	pc.Parameters = args
}
