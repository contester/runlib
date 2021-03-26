package subprocess

import (
	"os"
	"syscall"

	"github.com/contester/runlib/win32"
	"github.com/juju/errors"
)

func OpenFileForRedirect(name string, read bool) (*os.File, error) {
	var wmode, cmode uint32
	if read {
		wmode = syscall.GENERIC_READ
		cmode = syscall.OPEN_EXISTING
	} else {
		wmode = syscall.GENERIC_WRITE
		cmode = syscall.CREATE_ALWAYS
	}

	h, e := syscall.CreateFile(
		syscall.StringToUTF16Ptr(name),
		wmode,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil,
		cmode,
		win32.FILE_FLAG_SEQUENTIAL_SCAN,
		0)

	if e != nil {
		return nil, errors.Trace(os.NewSyscallError("CreateFile", e))
	}

	return os.NewFile(uintptr(h), name), nil
}

func ReaderDefault() (*os.File, error) {
	return nil, nil
}

func WriterDefault() (*os.File, error) {
	return nil, nil
}

func hackPipe() (r *os.File, w *os.File, err error) {
	var p [2]syscall.Handle
	e := syscall.CreatePipe(&p[0], &p[1], nil, 1024*1024*4)
	if e != nil {
		return nil, nil, os.NewSyscallError("pipe", e)
	}
	return os.NewFile(uintptr(p[0]), "|0"), os.NewFile(uintptr(p[1]), "|1"), nil
}
