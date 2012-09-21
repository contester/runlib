package platform

import (
	"runlib/win32"
	"syscall"
	"strconv"
	"os"
	l4g "code.google.com/p/log4go"
)

type ContesterDesktop struct {
	WindowStation win32.Hwinsta
	Desktop       win32.Hdesk
	DesktopName   string
}

type GlobalData struct {
	Desktop *ContesterDesktop
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

func CreateContesterDesktop() (winsta win32.Hwinsta, desk win32.Hdesk, name string, err error) {
	origWinsta, err := win32.GetProcessWindowStation()
	if err != nil {
		err = os.NewSyscallError("GetProcessWindowStation", err)
		return
	}
	origDesktop, err := win32.GetThreadDesktop(win32.GetCurrentThreadId())
	if err != nil {
		err = os.NewSyscallError("GetThreadDesktop", err)
		return
	}

	newName := "w" + strconv.FormatUint(uint64(win32.GetCurrentThreadId()), 10)

	newWinsta, err := win32.CreateWindowStation(syscall.StringToUTF16Ptr(newName), 0, win32.MAXIMUM_ALLOWED, win32.MakeInheritSa())
	if err != nil {
		err = os.NewSyscallError("CreateWindowStation", err)
		return
	}

	err = win32.SetProcessWindowStation(newWinsta)
	if err != nil {
		win32.CloseWindowStation(newWinsta)
		err = os.NewSyscallError("SetProcessWindowStation", err)
		return
	}

	winsta = newWinsta

	newWinstaName, err := win32.GetUserObjectName(syscall.Handle(newWinsta))

	if err == nil {
		shortName := "c" + strconv.FormatUint(uint64(win32.GetCurrentThreadId()), 10)

		desk, err = win32.CreateDesktop(
			syscall.StringToUTF16Ptr(shortName),
			nil, 0, 0, syscall.GENERIC_ALL, win32.MakeInheritSa())

		if err == nil {
			name = newWinstaName + "\\" + shortName
		} else {
			err = os.NewSyscallError("CreateDesktop", err)
		}
	} else {
		err = os.NewSyscallError("GetUserObjectName", err)
	}

	win32.SetProcessWindowStation(origWinsta)
	win32.SetThreadDesktop(origDesktop)

	everyone, err := syscall.StringToSid("S-1-1-0")
	if err == nil {
		err = win32.AddAceToWindowStation(newWinsta, everyone)
		if err != nil {
			l4g.Error(err)
		}
		err = win32.AddAceToDesktop(desk, everyone)
		if err != nil {
			l4g.Error(err)
		}
	} else {
		err = os.NewSyscallError("StringToSid", err)
	}

	return
}

func GetLoadLibrary() (uintptr, error) {
	handle, err := win32.GetModuleHandle(syscall.StringToUTF16Ptr("kernel32"))
	if err != nil {
		return 0, os.NewSyscallError("GetModuleHandle", err)
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
