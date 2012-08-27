package platform

import (
	"unsafe"
)

func AlignedBuffer(size, offset int) []byte {
	buf := make([]byte, size+offset)
	ofs := int((uintptr(offset) - (uintptr(unsafe.Pointer(&buf[0])) % uintptr(offset))) % uintptr(offset))
	return buf[ofs : ofs+size]
}
