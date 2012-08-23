package linux

// +build linux

/*
#cgo LDFLAGS: -lclonehelper
#include <stdlib.h>
#include "clone_helper.h"
*/
import "C"
import "unsafe"

const (
	alignOffset = 16
)

func startSubprocess(filename string, args, env []string, cwd string, suid int, stdhandles []int) {
	params := &C.struct_CloneParams{}
	tls := alignedBuffer(4096)
	stack := alignedBuffer(4096)
	params.tls = (*C.char)(unsafe.Pointer(&tls[0]))
	params.stack = (*C.char)(unsafe.Pointer(&stack[len(stack) - 1]))

	params.filename = C.CString(filename)
	defer C.free(unsafe.Pointer(params.filename))
}
