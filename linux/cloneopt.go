package linux

// +build linux

/*
#cgo LDFLAGS: -lclonehelper
#include <stdlib.h>
#include "clone_helper.h"
*/
import "C"
import "unsafe"
import "runlib/tools"
import "os"
import "runtime"
import "syscall"
import "fmt"

type StdHandles struct {
	StdIn, StdOut, StdErr *os.File
}

func (s *StdHandles) Close() {
	s.StdIn.Close()
	s.StdOut.Close()
	s.StdErr.Close()
}

type CloneParams struct {
	repr C.struct_CloneParams
	tls, stack []byte
	args, env []*C.char
	stdhandles StdHandles
	CommReader, CommWriter *os.File
}

func stringsToCchars(source []string) []*C.char {
	result := make([]*C.char, len(source) + 1)
	for i, v := range source {
		result[i] = C.CString(v)
	}
	return result
}

func deallocCchars(what []*C.char) {
	for _, v := range what {
		if v != nil {
			C.free(unsafe.Pointer(v))
		}
	}
}

func getFd(f *os.File) C.int32_t {
	if f != nil {
		return C.int32_t(f.Fd())
	}
	return -1
}

func CreateCloneParams(filename string, args, env []string, cwd string, suid int, stdhandles StdHandles) (*CloneParams, error) {
	result := &CloneParams{}
	var err error
	result.CommReader, result.CommWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	result.tls = tools.AlignedBuffer(4096, 16)
	result.stack = tools.AlignedBuffer(4096, 16)
	result.repr.tls = (*C.char)(unsafe.Pointer(&result.tls[0]))
	result.repr.stack = (*C.char)(unsafe.Pointer(&result.stack[len(result.stack) - 1]))
	result.repr.commfd = C.int32_t(result.CommWriter.Fd())
	
	result.repr.filename = C.CString(filename)
	if cwd != "" {
		result.repr.cwd = C.CString(cwd)
	}
	result.args = stringsToCchars(args)
	result.repr.argv = &result.args[0]
	result.env = stringsToCchars(env)
	result.repr.envp = &result.env[0]
	result.repr.suid = C.uint32_t(suid)
	result.stdhandles = stdhandles

	result.repr.stdhandles[0] = getFd(result.stdhandles.StdIn)
	result.repr.stdhandles[1] = getFd(result.stdhandles.StdOut)
	result.repr.stdhandles[2] = getFd(result.stdhandles.StdErr)

	runtime.SetFinalizer(result, freeCloneParams)
	return result, nil
}

func freeCloneParams(s *CloneParams) {
	if s.CommWriter != nil {
		s.CommWriter.Close()
	}
	if s.CommReader != nil {
		s.CommReader.Close()
	}
	s.stdhandles.Close()
	s.repr.tls = nil
	s.repr.stack = nil
	s.tls = nil
	s.stack = nil
	s.repr.argv = nil
	deallocCchars(s.args)
	s.args = nil
	s.repr.envp = nil
	deallocCchars(s.env)
	s.env = nil
	C.free(unsafe.Pointer(s.repr.filename))
	s.repr.filename = nil
	if s.repr.cwd != nil {
		C.free(unsafe.Pointer(s.repr.cwd))
		s.repr.cwd = nil
	}
}

func (c *CloneParams) Clone() (int, error) {
	pid := int(C.Clone(&c.repr))
	// TODO: clone errors?
	c.CommWriter.Close()
	c.stdhandles.Close()
	
	var status syscall.WaitStatus
	for {
		wpid, err := syscall.Wait4(pid, &status, 0, nil) // TODO: rusage
		if err != nil {
			return -1, err
		}
		if wpid == pid {
			break
		}
	}
	if status.Stopped() && status.StopSignal() == syscall.SIGTRAP {
		// cgroup attach
		err := syscall.PtraceDetach(pid)
		if err != nil {
			// wtf to do here
		}
		return pid, nil
	}
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		return -1, err
	}
	return -1, fmt.Errorf("traps, signals, dafuq is this")
}
