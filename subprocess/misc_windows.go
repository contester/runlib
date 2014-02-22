package subprocess

import (
	l4g "code.google.com/p/log4go"
	"github.com/contester/runlib/win32"
	"github.com/contester/runlib/tools"
	"runtime"
	"syscall"
	"unsafe"
)

// Loads user profile, using handle and username.
func loadProfile(user syscall.Handle, username string) (syscall.Handle, error) {
	ec := tools.ErrorContext("loadProfile")
	var pinfo win32.ProfileInfo
	var err error
	pinfo.Size = uint32(unsafe.Sizeof(pinfo))
	pinfo.Flags = win32.PI_NOUI
	pinfo.Username, err = syscall.UTF16PtrFromString(username)
	if err != nil {
		return syscall.InvalidHandle, ec.NewError(err, ERR_USER, "UTF16PtrFromString")
	}

	err = win32.LoadUserProfile(user, &pinfo)
	if err != nil {
		l4g.Trace("Error loading profile for %d/%s", user, username)
		return syscall.InvalidHandle, ec.NewError(err, "LoadUserProfile")
	}
	return pinfo.Profile, nil
}

// Log user out, unloading profiles if necessary.
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

// Finalizer, calls realLogout in goroutine.
func logout(s *LoginInfo) {
	go realLogout(s)
}

// Login and load user profile. Also, set finalizer on s to logout() above.
func (s *LoginInfo) Prepare() error {
	var err error
	if s.Username == "" {
		return nil
	}

	ec := tools.ErrorContext("LoginInfo.Prepare")

	s.HUser, err = win32.LogonUser(
		syscall.StringToUTF16Ptr(s.Username),
		syscall.StringToUTF16Ptr("."),
		syscall.StringToUTF16Ptr(s.Password),
		win32.LOGON32_LOGON_INTERACTIVE,
		win32.LOGON32_PROVIDER_DEFAULT)

	if err != nil {
		return ec.NewError(err)
	}

	s.HProfile, err = loadProfile(s.HUser, s.Username)

	if err != nil {
		syscall.CloseHandle(s.HUser)
		s.HUser = syscall.InvalidHandle
		return ec.NewError(err)
	}

	runtime.SetFinalizer(s, logout)
	return nil
}
