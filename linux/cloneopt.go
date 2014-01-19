package linux

// +build linux

/*
#cgo linux,386 LDFLAGS: -lpthread -lrt -lcap
#cgo linux,amd64 LDFLAGS: -lpthread -lrt -lcap
#include <stdlib.h>
#include "clone_helper.h"
*/
import "C"
import "unsafe"
import "github.com/contester/runlib/tools"
import "os"
import "runtime"

type CloneParams struct {
	repr                   C.struct_CloneParams
	tls, stack             []byte
	args, env              []*C.char
	stdhandles             StdHandles
	CommReader, CommWriter *os.File
	comm                   chan CommStatus
}

func stringsToCchars(source []string) []*C.char {
	result := make([]*C.char, len(source)+1)
	for i, v := range source {
		result[i] = C.CString(v)
	}
	return result
}

func deallocCchars(what []*C.char) {
	if what == nil {
		return
	}
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

func CreateCloneParams(filename string, args []string, env *[]string, cwd *string, suid int, stdhandles StdHandles) (*CloneParams, error) {
	result := &CloneParams{}
	var err error
	result.CommReader, result.CommWriter, err = os.Pipe()
	if err != nil {
		return nil, err
	}
	result.tls = tools.AlignedBuffer(4096, 16)
	result.stack = tools.AlignedBuffer(4096, 16)
	result.repr.tls = (*C.char)(unsafe.Pointer(&result.tls[0]))
	result.repr.stack = (*C.char)(unsafe.Pointer(&result.stack[len(result.stack)-1]))
	result.repr.commfd = C.int32_t(result.CommWriter.Fd())

	result.repr.filename = C.CString(filename)
	if cwd != nil {
		result.repr.cwd = C.CString(*cwd)
	}
	if args != nil {
		result.args = stringsToCchars(args)
		result.repr.argv = &result.args[0]
	}
	if env != nil {
		result.env = stringsToCchars(*env)
		result.repr.envp = &result.env[0]
	}
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

func callClone(c *CloneParams) int {
	return int(C.Clone(&c.repr))
}
