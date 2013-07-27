package subprocess

import (
	"bytes"
	"io"
	"os"

	"github.com/contester/runlib/tools"
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
		return nil, tools.NewComponentError(e, "SetupOutputMemory", "os.Pipe")
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
		return nil, tools.NewComponentError(e, "SetupFile", "OpenFile")
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
		return nil, tools.NewComponentError(e, "SetupInputMemory", "os.Pipe")
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

func recordingTee(w io.WriteCloser, r io.ReadCloser, t io.Writer) {
	m := io.MultiWriter(w, t)
	io.Copy(m, r)
	w.Close()
	r.Close()
}

// In functions below, we are forced to use *os.File instead of, say, io.Writer
// for the reasons mentioned in http://golang.org/doc/go_faq.html#nil_error
// I could work around it by using reflection, but why...

func RecordingPipe(d *os.File) (*os.File, *os.File, error) {
	if d == nil {
		return os.Pipe()
	}

	r1, w1, e := os.Pipe()
	if e != nil {
		return nil, nil, e
	}

	r2, w2, e := os.Pipe()
	if e != nil {
		return nil, nil, e
	}

	go recordingTee(w1, r2, d)

	return r1, w2, nil
}

func Interconnect(s1, s2 *Subprocess, d1, d2 *os.File) error {
	ec := tools.NewContext("Interconnect")

	read1, write1, err := RecordingPipe(d1)
	if err != nil {
		return ec.NewError(err, "RecordingPipe")
	}

	read2, write2, err := RecordingPipe(d2)
	if err != nil {
		read1.Close()
		write1.Close()
		return ec.NewError(err, "RecordingPipe")
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
