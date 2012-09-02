package linux

// +build linux

/*
#cgo LDFLAGS: -lclonehelper
#include <stdlib.h>
#include "clone_helper.h"
*/
import "C"
import "unsafe"
import "runlib/tools""

type StdHandles struct {
	StdIn, StdOut, StdErr *os.File
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

func CreateCloneParams(filename string, args, env []string, cwd string, suid int, stdhandles StdHandles) (*CloneParams, error) {
	result := &CloneParams{}
	var err error
	result.CommReader, result.CommWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	result.tls = tools.AlignedBuffer(4096, 16)
	result.stack = tools.AlignedBuffer(4096, 16)
	result.repr.tls = (*C.char)(unsafe.Pointer(&tls[0]))
	result.repr.stack = (*C.char)(unsafe.Pointer(&stack[len(stack) - 1]))
	result.repr.commfd = result.CommWriter.Fd()
	
	params.repr.filename = C.CString(filename)
	if cwd != "" {
		params.repr.cwd = C.CString(cwd)
	}
	result.args = stringsToCchars(args)
	result.repr.argv = &result.args[0]
	result.env = stringsToCchars(env)
	result.repr.envp = &result.env[0]
	result.repr.suid = suid
	result.repr.stdhandles[0] = stdhandles.StdIn
	result.repr.stdhandles[1] = stdhandles.StdOut
	result.repr.stdhandles[2] = stdhandles.StdErr

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
	s.repr.tls = nil
	s.repr.stack = nil
	s.tls = nil
	s.stack = nil
	s.repr.argv = nil
	deallocChars(s.args)
	s.args = nil
	s.repr.envp = nil
	deallocChars(s.env)
	s.env = nil
	C.free(unsafe.Pointer(s.repr.filename)
	s.repr.filename = nil
	if s.repr.cwd != nil {
		C.free(unsafe.Pointer(s.repr.cwd))
		s.repr.cwd = nil
	}
}


func (c *CloneParams) Clone() (int, error) {
	pid = C.Clone(&c.repr)
	// TODO: clone errors?
	c.CommWriter.Close()
	
	for {
		var buf [2]int32_t
		io.ReadFull
	}
}
