package subprocess

import (
	"bytes"
	"io"
	"os"
	"runlib/platform/win32"
	"syscall"
	"unsafe"
)

type Redirect struct {
	Mode     int
	Filename *string
	Pipe   *os.File
	Data     []byte
}

func (d *subprocessData) SetupOutputMemory(b *bytes.Buffer) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, e
	}

	e = win32.SetInheritHandle(syscall.Handle(reader.Fd()), false)
	if e != nil {
		return nil, e
	}

	e = win32.SetInheritHandle(syscall.Handle(writer.Fd()), true)
	if e != nil {
		return nil, e
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)

	d.startAfterStart = append(d.startAfterStart, func() error {
		_, err := io.Copy(b, reader)
		reader.Close()
		return err
	})
	return writer, nil
}

func (d *subprocessData) SetupFile(filename string, read bool) (*os.File, error) {
	writer, e := OpenFileForRedirect(filename, read)
	if e != nil {
		return nil, e
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)
	return writer, nil
}

func (d *subprocessData) SetupPipe(f *os.File) (*os.File, error) {
	d.closeAfterStart = append(d.closeAfterStart, f)
	return f, nil
}

func (d *subprocessData) SetupOutput(w *Redirect, b *bytes.Buffer) (*os.File, error) {
	if w == nil {
		return nil, nil
	}

	switch w.Mode {
	case REDIRECT_MEMORY:
		return d.SetupOutputMemory(b)
	case REDIRECT_FILE:
		return d.SetupFile(*w.Filename, false)
	case REDIRECT_PIPE:
		return d.SetupPipe(w.Pipe)
	}
	return nil, nil
}

func (d *subprocessData) SetupInputMemory(b []byte) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	d.closeAfterStart = append(d.closeAfterStart, reader)
	d.startAfterStart = append(d.startAfterStart, func() error {
		_, err := io.Copy(writer, bytes.NewBuffer(b))
		if err1 := writer.Close(); err == nil {
			err = err1
		}
		return err
	})
	return reader, nil
}

func (d *subprocessData) SetupInput(w *Redirect) (*os.File, error) {
	if w == nil {
		return nil, nil
	}

	switch w.Mode {
	case REDIRECT_MEMORY:
		return d.SetupInputMemory(w.Data)
	case REDIRECT_PIPE:
		return d.SetupPipe(w.Pipe)
	case REDIRECT_FILE:
		return d.SetupFile(*w.Filename, true)
	}
	return nil, nil
}

func OpenFileForRedirect(name string, read bool) (*os.File, error) {
	sa := &syscall.SecurityAttributes{}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	sa.InheritHandle = 1

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
		sa,
		cmode,
		win32.FILE_FLAG_SEQUENTIAL_SCAN,
		0)

	if e != nil {
		return nil, e
	}

	return os.NewFile(uintptr(h), name), nil
}
