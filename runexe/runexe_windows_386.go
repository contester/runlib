package main

import (
	"runlib/platform"
	"runlib/subprocess"
	"strings"
)

func CreateDesktopIfNeeded(program, interactor *ProcessConfig) (*platform.ContesterDesktop, error) {
	if !program.NeedLogin() && (interactor != nil && !interactor.NeedLogin()) {
		return nil, nil
	}

	return platform.CreateContesterDesktopStruct()
}

func GetLoadLibraryIfNeeded(program, interactor *ProcessConfig) (uintptr, error) {
	if program.InjectDLL == "" && (interactor == nil || interactor.InjectDLL == "") {
		return 0, nil
	}
	return platform.GetLoadLibrary()
}

func setDesktop(p *subprocess.PlatformOptions, desktop *platform.ContesterDesktop) {
	if desktop != nil {
		p.Desktop = desktop.DesktopName
	}
}

func setInject(p *subprocess.PlatformOptions, injectDll string, loadLibraryW uintptr) {
	if injectDll != "" && loadLibraryW != 0 {
		p.InjectDLL = injectDll
		p.LoadLibraryW = loadLibraryW
	}
}

func ArgsToPc(pc *ProcessConfig, args []string) {
	pc.CommandLine = strings.Join(args, " ")
	pc.Parameters = args
}
