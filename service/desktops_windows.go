package service

import (
	"runlib/platform/win32"
	"syscall"
	l4g "code.google.com/p/log4go"

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
	}

	// win32.CloseWindowStation(newWinsta)
	return
}
