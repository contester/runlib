package win32

// +build windows,386

import (
	"syscall"
	"unsafe"
)

var (
	procGetUserObjectSecurity = user32.NewProc("GetUserObjectSecurity")
)



func GetUserObjectSecurity(obj syscall.Handle, sid uint32, desc []byte) (uint32, error) {
	var nLength uint32
	r1, _, e1 := procGetUserObjectSecurity.Call(
		uintptr(obj),
		uintptr(unsafe.Pointer(&sid)),
		uintptr(unsafe.Pointer(&desc[0])),
		uintptr(len(desc)),
		uintptr(unsafe.Pointer(&nLength)))
	if int(r1) == 0 {
		return nLength, e1
	}
	return nLength, nil
}