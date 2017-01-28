package subprocess

import (
	"syscall"
	"os"
	"github.com/juju/errors"
)

func IsUserError(err error) bool {
	return errors.IsBadRequest(err)
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