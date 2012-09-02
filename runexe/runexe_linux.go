package main

import (
	"runlib/platform"
	"runlib/subprocess"
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
