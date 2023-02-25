package subprocess

import (
	"errors"
	"os"
	"syscall"
)

var ErrUserError = errors.New("user error")

func IsUserError(err error) bool {
	return errors.Is(err, ErrUserError)
}

func extractErrno(e error) (syscall.Errno, bool) {
	if e == nil {
		return 0, false
	}
	if e2, ok := e.(*os.SyscallError); ok {
		e = e2.Err
	}
	if errno, ok := e.(syscall.Errno); ok {
		return errno, true
	}
	return 0, false
}
