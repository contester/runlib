package subprocess

import (
	"os"
	"syscall"

	"github.com/contester/runlib/win32"
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
		return nil, e
	}

	return os.NewFile(uintptr(h), name), nil
}

func ReaderDefault() (*os.File, error) {
	return nil, nil
}

func WriterDefault() (*os.File, error) {
	return nil, nil
}
