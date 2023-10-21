package win32

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modnetapi = windows.NewLazySystemDLL("netapi32.dll")

	netUserAddProc     = modnetapi.NewProc("NetUserAdd")
	netUserSetInfoProc = modnetapi.NewProc("NetUserSetInfo")
)

type rawUserInfo1 struct {
	usri1_name         *uint16
	usri1_password     *uint16
	usri1_password_age uint32
	usri1_priv         uint32
	usri1_home_dir     *uint16
	usri1_comment      *uint16
	usri1_flags        uint32
	usri1_script_path  *uint16
}

type userInfo1003 struct {
	password *uint16
}

const UF_PASSWD_CANT_CHANGE = 0x0040
const UF_DONT_EXPIRE_PASSWD = 0x10000

// AddLocalUser adds a local user account with the given username and password.
// The function returns an error if the user account already exists or if there
// is an issue with the system call.
func AddLocalUser(username, password string) error {
	u16, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	p16, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return err
	}
	uStruct := &rawUserInfo1{
		usri1_name:     u16,
		usri1_password: p16,
		usri1_priv:     1, // USER_PRIV_USER
		usri1_flags:    UF_PASSWD_CANT_CHANGE | UF_DONT_EXPIRE_PASSWD,
	}

	ret, _, _ := netUserAddProc.Call(
		0, // local computer
		1,
		uintptr(unsafe.Pointer(uStruct)),
		0,
	)

	runtime.KeepAlive(uStruct)

	if ret != 0 {
		return syscall.Errno(ret)
	}

	return nil
}

// IsAccountAlreadyExists returns true if the error is caused by an existing user account.
func IsAccountAlreadyExists(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == 0x000008B0
	}
	return false
}

// SetLocalUserPassword sets the password for the given local user account.
func SetLocalUserPassword(username, password string) error {
	u16, err := syscall.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	p16, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return err
	}

	uStruct := &userInfo1003{password: p16}

	ret, _, _ := netUserSetInfoProc.Call(
		0, // local computer
		uintptr(unsafe.Pointer(u16)),
		1003,
		uintptr(unsafe.Pointer(uStruct)),
		0,
	)

	runtime.KeepAlive(uStruct)
	runtime.KeepAlive(u16)

	if ret != 0 {
		return syscall.Errno(ret)
	}

	return nil
}
