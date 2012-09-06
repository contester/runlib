package subprocess

import (
	"bytes"
	"io"
	"os"
)

type Redirect struct {
	Mode     int
	Filename *string
	Pipe     *os.File
	Data     []byte
}

func (d *SubprocessData) SetupOutputMemory(b *bytes.Buffer) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, NewSubprocessError(false, "SetupOutputMemory/os.Pipe", e)
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)

	d.startAfterStart = append(d.startAfterStart, func() error {
		_, err := io.Copy(b, reader)
		reader.Close()
		return err
	})
	return writer, nil
}

func (d *SubprocessData) SetupFile(filename string, read bool) (*os.File, error) {
	writer, e := OpenFileForRedirect(filename, read)
	if e != nil {
		return nil, NewSubprocessError(false, "SetupFile/OpenFile", e)
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)
	return writer, nil
}

func (d *SubprocessData) SetupPipe(f *os.File) (*os.File, error) {
	d.closeAfterStart = append(d.closeAfterStart, f)
	return f, nil
}

func (d *SubprocessData) SetupOutput(w *Redirect, b *bytes.Buffer) (*os.File, error) {
	if w == nil {
		return WriterDefault()
	}

	switch w.Mode {
	case REDIRECT_MEMORY:
		return d.SetupOutputMemory(b)
	case REDIRECT_FILE:
		return d.SetupFile(*w.Filename, false)
	case REDIRECT_PIPE:
		return d.SetupPipe(w.Pipe)
	}
	return WriterDefault()
}

func (d *SubprocessData) SetupInputMemory(b []byte) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, NewSubprocessError(false, "SetupInputMemory/os.Pipe", e)
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

func (d *SubprocessData) SetupInput(w *Redirect) (*os.File, error) {
	if w == nil {
		return ReaderDefault()
	}

	switch w.Mode {
	case REDIRECT_MEMORY:
		return d.SetupInputMemory(w.Data)
	case REDIRECT_PIPE:
		return d.SetupPipe(w.Pipe)
	case REDIRECT_FILE:
		return d.SetupFile(*w.Filename, true)
	}
	return ReaderDefault()
}

func Interconnect(s1, s2 *Subprocess) error {
	read1, write1, err := os.Pipe()
	if err != nil {
		return NewSubprocessError(false, "Interconnect/os.Pipe", err)
	}
	read2, write2, err := os.Pipe()
	if err != nil {
		read1.Close()
		read2.Close()
		return NewSubprocessError(false, "Interconnect/os.Pipe", err)
	}

	s1.StdIn = &Redirect{
		Mode: REDIRECT_PIPE,
		Pipe: read1,
	}
	s2.StdOut = &Redirect{
		Mode: REDIRECT_PIPE,
		Pipe: write1,
	}
	s1.StdOut = &Redirect{
		Mode: REDIRECT_PIPE,
		Pipe: write2,
	}
	s2.StdIn = &Redirect{
		Mode: REDIRECT_PIPE,
		Pipe: read2,
	}
	return nil
}
