package service

import (
	"runlib/platform/win32"
	"syscall"

//	"fmt"
)

func CreateContesterDesktop() (desk win32.Hdesk, name string, err error) {
	origWinsta, err := win32.GetProcessWindowStation()
	if err != nil {
		return
	}
	origDesktop, err := win32.GetThreadDesktop(win32.GetCurrentThreadId())
	if err != nil {
		return
	}

	newWinsta, err := win32.CreateWindowStation(nil, 0, win32.MAXIMUM_ALLOWED, win32.MakeInheritSa())
	if err != nil {
		return
	}

	err = win32.SetProcessWindowStation(newWinsta)
	if err != nil {
		win32.CloseWindowStation(newWinsta)
		return
	}

	newWinstaName, err := win32.GetUserObjectName(syscall.Handle(newWinsta))

	if err == nil {
		desk, err = win32.CreateDesktop(
			syscall.StringToUTF16Ptr("contester"),
			nil, 0, 0, syscall.GENERIC_ALL, win32.MakeInheritSa())

		if err == nil {
			name = newWinstaName + "\\contester"
		}
	}

	win32.SetProcessWindowStation(origWinsta)
	win32.SetThreadDesktop(origDesktop)
	win32.CloseWindowStation(newWinsta)
	return
}
