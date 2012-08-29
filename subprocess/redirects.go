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

func (d *subprocessData) SetupOutputFile(filename string) (*os.File, error) {
	writer, e := OpenFileForOutputRedirect(filename)
	if e != nil {
		return nil, e
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)
	return writer, nil
}

func (d *subprocessData) SetupOutputPipe(f *os.File) (*os.File, error) {
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
		return d.SetupOutputFile(*w.Filename)
	case REDIRECT_PIPE:
		return d.SetupOutputPipe(w.Pipe)
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
	}
	return nil, nil
}

func OpenFileForOutputRedirect(name string) (*os.File, error) {
	sa := &syscall.SecurityAttributes{}
	sa.Length = uint32(unsafe.Sizeof(*sa))
	sa.InheritHandle = 1

	h, e := syscall.CreateFile(
		syscall.StringToUTF16Ptr(name),
		syscall.GENERIC_WRITE,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		sa,
		syscall.CREATE_ALWAYS,
		win32.FILE_FLAG_SEQUENTIAL_SCAN,
		0)

	if e != nil {
		return nil, e
	}

	return os.NewFile(uintptr(h), name), nil
}
