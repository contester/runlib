package main

import "runlib/platform"

func CreateDesktopIfNeeded(program, interactor *ProcessConfig) (*platform.ContesterDesktop, error) {
	return nil, nil
}

func GetLoadLibraryIfNeeded(program, interactor *ProcessConfig) (uintptr, error) {
return nil,nil
}

func setDesktop(p *subprocess.PlatformOptions, desktop *platform.ContesterDesktop) {
}

func setInject(p *subprocess.PlatformOptions, injectDll string, loadLibraryW uintptr) {
}
