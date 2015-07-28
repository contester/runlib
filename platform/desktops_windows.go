package platform

import (
	"os"
	"strconv"
	"syscall"
	"runtime"

	"github.com/contester/runlib/win32"
	log "github.com/Sirupsen/logrus"
)

type ContesterDesktop struct {
	WindowStation win32.Hwinsta
	Desktop       win32.Hdesk
	DesktopName   string
}

type GlobalData struct {
	Desktop      *ContesterDesktop
	LoadLibraryW uintptr
}

func CreateContesterDesktopStruct() (*ContesterDesktop, error) {
	var result ContesterDesktop
	var err error
	result.WindowStation, result.Desktop, result.DesktopName, err = CreateContesterDesktop()
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func threadIdName(prefix string) string {
	return prefix + strconv.FormatUint(uint64(win32.GetCurrentThreadId()), 10)
}

func CreateContesterDesktop() (winsta win32.Hwinsta, desk win32.Hdesk, name string, err error) {
	var origWinsta win32.Hwinsta
	if origWinsta, err = win32.GetProcessWindowStation(); err != nil {
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var origDesktop win32.Hdesk
	if origDesktop, err = win32.GetThreadDesktop(win32.GetCurrentThreadId()); err != nil {
		return
	}

	if winsta, err = win32.CreateWindowStation(
		syscall.StringToUTF16Ptr(threadIdName("w")), 0, win32.MAXIMUM_ALLOWED, win32.MakeInheritSa()); err != nil {
		return
	}

	if err = win32.SetProcessWindowStation(winsta); err != nil {
		win32.CloseWindowStation(winsta)
		return
	}

	var winstaName string
	if winstaName, err = win32.GetUserObjectName(syscall.Handle(winsta)); err == nil {
		shortName := threadIdName("c")

		desk, err = win32.CreateDesktop(
			syscall.StringToUTF16Ptr(shortName),
			nil, 0, 0, syscall.GENERIC_ALL, win32.MakeInheritSa())

		if err == nil {
			name = winstaName + "\\" + shortName
		}
	}

	win32.SetProcessWindowStation(origWinsta)
	win32.SetThreadDesktop(origDesktop)

	if err != nil {
		return
	}

	everyone, err := syscall.StringToSid("S-1-1-0")
	if err == nil {
		if err = win32.AddAceToWindowStation(winsta, everyone); err != nil {
			log.Error(err)
		}
		if err = win32.AddAceToDesktop(desk, everyone); err != nil {
			log.Error(err)
		}
	} else {
		err = os.NewSyscallError("StringToSid", err)
	}

	return
}

func GetLoadLibrary() (uintptr, error) {
	handle, err := win32.GetModuleHandle(syscall.StringToUTF16Ptr("kernel32"))
	if err != nil {
		return 0, err
	}
	addr, err := syscall.GetProcAddress(handle, "LoadLibraryW")
	if err != nil {
		return 0, os.NewSyscallError("GetProcAddress", err)
	}
	return addr, nil
}

func CreateGlobalData() (*GlobalData, error) {
	var err error
	var result GlobalData
	result.Desktop, err = CreateContesterDesktopStruct()

	if err != nil {
		return nil, err
	}

	result.LoadLibraryW, err = GetLoadLibrary()
	if err != nil {
		return nil, err
	}
	return &result, nil
}
