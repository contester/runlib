// +build windows,386

package subprocess

import (
	l4g "code.google.com/p/log4go"
	"runlib/win32"
	"runtime"
	"syscall"
	"unsafe"
)

func loadProfile(user syscall.Handle, username string) (syscall.Handle, error) {
	var pinfo win32.ProfileInfo
	pinfo.Size = uint32(unsafe.Sizeof(pinfo))
	pinfo.Flags = win32.PI_NOUI
	pinfo.Username = syscall.StringToUTF16Ptr(username)

	err := win32.LoadUserProfile(user, &pinfo)
	if err == nil {
		return syscall.InvalidHandle, err
	}
	return pinfo.Profile, nil
}

func realLogout(s *LoginInfo) {
	if s.HProfile != syscall.Handle(0) && s.HProfile != syscall.InvalidHandle {
		for {
			err := win32.UnloadUserProfile(s.HUser, s.HProfile)
			if err == nil {
				break
			}
			l4g.Error(err)
		}
		s.HProfile = syscall.InvalidHandle
	}

	if s.HUser != syscall.Handle(0) && s.HUser != syscall.InvalidHandle {
		syscall.CloseHandle(s.HUser)
		s.HUser = syscall.InvalidHandle
	}
}

func logout(s *LoginInfo) {
	go realLogout(s)
}

func (s *LoginInfo) Prepare() error {
	var err error
	if s.Username == "" {
		return nil
	}

	s.HUser, err = win32.LogonUser(
		syscall.StringToUTF16Ptr(s.Username),
		syscall.StringToUTF16Ptr("."),
		syscall.StringToUTF16Ptr(s.Password),
		win32.LOGON32_LOGON_INTERACTIVE,
		win32.LOGON32_PROVIDER_DEFAULT)

	if err != nil {
		return err
	}

	s.HProfile, err = loadProfile(s.HUser, s.Username)

	if err != nil {
		syscall.CloseHandle(s.HUser)
		s.HUser = syscall.InvalidHandle
		return err
	}

	runtime.SetFinalizer(s, logout)
	return nil
}
