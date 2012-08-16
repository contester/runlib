package subprocess

import (
	"bytes"
)

type Redirect struct {
	Mode     int
	Filename *string
	Handle   uintptr
	Data     []byte
}

func (d *subprocessData) SetupOutputMemory(b *bytes.Buffer) (os.File, error) {
	reader, writer, e := os.Pipe()
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

func (d *subprocessData) SetupOutputFile(filename string) (os.File, error) {
	writer, e := OpenFileForOutputRedirect(filename)
	if e != nil {
		return nil, e
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)
	return writer, nil
}

func (d *subprocessData) SetupOutput(w *Redirect, b *bytes.Buffer) (os.File, error) {
	if w == nil {
		return nil, nil
	}

	switch w.Mode {
	case MODE_MEMORY:
		return d.SetupOutputMemory(b)
	case MODE_FILE:
		return d.SetupOutputFile(w.Filename)
	}
	return nil, nil
}

func (d *subprocessData) SetupInputMemory(b []byte) (os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, e
	}
	d.closeAfterStart = append(d.closeAfterStart, reader)
	d.startAfterStart = append(d.startAfterStart, func() error {
		_, err := io.Copy(writer, &bytes.NewBuffer(b))
		if err1 := writer.Close(); err == nil {
			err = err1
		}
		return err
	})
	return reader, nil
}

func (d *subprocessData) SetupInput(w *Redirect) (os.File, error) {
	if w == nil {
		return nil, nil
	}

	switch w.Mode {
	case MODE_MEMORY:
		return d.SetupInputMemory(w.Data)
	}
	return nil, nil
}
