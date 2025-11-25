package subprocess

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Redirect struct {
	Mode     RedirectMode
	Filename string
	Pipe     *os.File
	Data     []byte

	MaxOutputSize int64
}

const MAX_MEM_OUTPUT = 1024 * 1024 * 1024

type PipeResultRecorder interface {
	Record(direction int, numBytes int64, err error)
}

type PipeRecordEntry struct {
	Direction int
	Timestamp time.Time
	Bytes     int64
	Error     error
}

type OrderedRecorder struct {
	mu      sync.RWMutex
	entries []PipeRecordEntry
}

func (s *OrderedRecorder) Record(direction int, numBytes int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, PipeRecordEntry{
		Direction: direction,
		Timestamp: time.Now(),
		Bytes:     numBytes,
		Error:     err,
	})
}

func (s *OrderedRecorder) GetEntries() []PipeRecordEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.entries
}

func (d *SubprocessData) SetupOutputMemory(b *bytes.Buffer, maxOutputSize int64) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, fmt.Errorf("SetupOutputMemory: os.Pipe: %w", e)
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)

	if maxOutputSize <= 0 {
		maxOutputSize = MAX_MEM_OUTPUT
	}

	d.startAfterStart = append(d.startAfterStart, func() error {
		_, err := io.Copy(b, io.LimitReader(reader, maxOutputSize))
		reader.Close()
		return err
	})

	d.cleanupIfFailed = append(d.cleanupIfFailed, func() {
		reader.Close()
	})
	return writer, nil
}

func (d *SubprocessData) SetupFile(filename string, read bool, maxOutputSize int64, isStdErr bool) (*os.File, error) {
	writer, e := OpenFileForRedirect(filename, read)
	if e != nil {
		return nil, e
	}

	d.closeAfterStart = append(d.closeAfterStart, writer)

	if maxOutputSize < 0 || read {
		return writer, nil
	}

	if maxOutputSize == 0 {
		maxOutputSize = MAX_MEM_OUTPUT
	}

	wcheck, err := OpenFileForCheck(filename)
	if err != nil {
		writer.Close()
		return nil, fmt.Errorf("opening %q for size check: %w", filename, err)
	}

	cw := &outputRedirectCheck{
		n:       filename,
		f:       wcheck,
		maxSize: maxOutputSize,
	}

	if isStdErr {
		d.errCheck = cw
	} else {
		d.outCheck = cw
	}

	return writer, nil
}

func (d *SubprocessData) SetupPipe(f *os.File) (*os.File, error) {
	d.closeAfterStart = append(d.closeAfterStart, f)
	return f, nil
}

func (d *SubprocessData) SetupOutput(w *Redirect, b *bytes.Buffer, isStdErr bool) (*os.File, error) {
	if w == nil {
		return WriterDefault()
	}

	switch w.Mode {
	case REDIRECT_MEMORY:
		return d.SetupOutputMemory(b, w.MaxOutputSize)
	case REDIRECT_FILE:
		return d.SetupFile(w.Filename, false, w.MaxOutputSize, isStdErr)
	case REDIRECT_PIPE:
		return d.SetupPipe(w.Pipe)
	}
	return WriterDefault()
}

func (d *SubprocessData) SetupInputMemory(b []byte) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, fmt.Errorf("SetupInputMemory: os.Pipe: %w", e)
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

func (d *SubprocessData) SetupInputRemote(r io.ReadCloser) (*os.File, error) {
	reader, writer, e := os.Pipe()
	if e != nil {
		return nil, fmt.Errorf("SetupInputRemote: os.Pipe: %w", e)
	}

	// TODO: rewrite resource management to not leak on startup failure.
	d.closeAfterStart = append(d.closeAfterStart, reader)
	d.startAfterStart = append(d.startAfterStart, func() error {
		defer r.Close()
		_, err := io.Copy(writer, r)
		if err1 := writer.Close(); err == nil {
			err = err1
		}
		return err
	})
	d.cleanupIfFailed = append(d.cleanupIfFailed, func() {
		r.Close()
		writer.Close()
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
		return d.SetupFile(w.Filename, true, 0, false)
	}
	return ReaderDefault()
}

func recordingTee(w io.WriteCloser, r io.ReadCloser, t io.Writer, recorder func(int64, error)) {
	defer r.Close()
	defer w.Close()

	var wc io.Writer = w

	if t != nil {
		wc = io.MultiWriter(t, w)
	}
	n, err := io.Copy(wc, r)
	if recorder != nil {
		recorder(n, err)
	}
}

func RecordingPipe(d io.Writer, recorder func(int64, error)) (*os.File, *os.File, error) {
	if d == nil && recorder == nil {
		return hackPipe()
	}

	r1, w1, e := hackPipe()
	if e != nil {
		return nil, nil, fmt.Errorf("RecordingPipe: first hackPipe: %w", e)
	}

	r2, w2, e := hackPipe()
	if e != nil {
		r1.Close()
		w1.Close()
		return nil, nil, fmt.Errorf("RecordingPipe: second hackPipe: %w", e)
	}

	var t io.Writer
	if d != nil {
		t = d
	}

	go recordingTee(w1, r2, t, recorder)

	return r1, w2, nil
}

func recordDirection(recorder PipeResultRecorder, direction int) func(int64, error) {
	if recorder == nil {
		return nil
	}
	return func(n int64, err error) {
		recorder.Record(direction, n, err)
	}
}

type SyncWriter struct {
	w     io.Writer
	Mutex sync.RWMutex
}

func (w *SyncWriter) Write(p []byte) (n int, err error) {
	// w.Mutex.Lock()
	// defer w.Mutex.Unlock()
	n, err = w.w.Write(p)
	return n, err
}

func Interconnect(s1, s2 *Subprocess, d1, d2 *os.File, recorder PipeResultRecorder) error {
	writer := &SyncWriter{w: d1}

	read1, write1, err := RecordingPipe(writer, recordDirection(recorder, 0))
	if err != nil {
		return err
	}

	read2, write2, err := RecordingPipe(writer, recordDirection(recorder, 1))
	if err != nil {
		read1.Close()
		write1.Close()
		return err
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
