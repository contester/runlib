package subprocess

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/contester/runlib/win32"
	"github.com/juju/errors"

	log "github.com/sirupsen/logrus"
)

// Loads user profile, using handle and username.
func loadProfile(user syscall.Handle, username string) (syscall.Handle, error) {
	var pinfo win32.ProfileInfo
	var err error
	pinfo.Size = uint32(unsafe.Sizeof(pinfo))
	pinfo.Flags = win32.PI_NOUI
	pinfo.Username, err = syscall.UTF16PtrFromString(username)
	if err != nil {
		return syscall.InvalidHandle, errors.NewBadRequest(err, fmt.Sprintf("UTF16PtrFromString(%q)", username))
	}

	err = win32.LoadUserProfile(user, &pinfo)
	if err != nil {
		log.Debug("Error loading profile for %d/%s", user, username)
		return syscall.InvalidHandle, errors.Annotatef(err, "LoadUserProfile(%q, %+v)", user, &pinfo)
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
			log.Error(err)
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
	if s.Username == "" {
		return nil
	}

	var err error
	s.HUser, err = win32.LogonUser(
		s.Username,
		".",
		s.Password,
		win32.LOGON32_LOGON_INTERACTIVE,
		win32.LOGON32_PROVIDER_DEFAULT)

	if err != nil {
		return errors.Trace(err)
	}

	s.HProfile, err = loadProfile(s.HUser, s.Username)

	if err != nil {
		syscall.CloseHandle(s.HUser)
		s.HUser = syscall.InvalidHandle
		return errors.Trace(err)
	}

	runtime.SetFinalizer(s, logout)
	return nil
}
