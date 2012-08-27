package win32

// +build windows,386

import (
	"syscall"
	"unsafe"
	"runlib/platform"
)

var (
	procGetUserObjectSecurity = user32.NewProc("GetUserObjectSecurity")
	procGetSecurityDescriptorDacl = advapi32.NewProc("GetSecurityDescriptorDacl")
	procIsValidAcl = advapi32.NewProc("IsValidAcl")
	procGetAclInformation = advapi32.NewProc("GetAclInformation")
	procInitializeSecurityDescriptor = advapi32.NewProc("InitializeSecurityDescriptor")
	procInitializeAcl = advapi32.NewProc("InitializeAcl")
	procAddAce = advapi32.NewProc("AddAce")
)

const (
	DACL_SECURITY_INFORMATION = 0x00000004
	SECURITY_DESCRIPTOR_REVISION = 1
	ACL_REVISION = 2
)

func AddAceToDesktop(desk Hdesk, sid *syscall.SID) {
	secDesc, err := GetUserObjectSecurity(syscall.Handle(desk), DACL_SECURITY_INFORMATION)
	_, acl, _, err := GetSecurityDescriptorDacl(secDesc)

	newDesc, err := CreateSecurityDescriptor(256)
	var aclSize *AclSizeInformation
	if acl != nil {
		aclSize, err = GetAclSize(acl)
	}
	newAcl, err := CreateNewAcl(1024)

	
}	

func AddAceToWindowStation(winsta Hwinsta, sid *syscall.SID) error {
	
	return nil
}


func CreateSecurityDescriptor(length int) ([]byte, error) {
	result := platform.AlignedBuffer(length, 4)
	err := InitializeSecurityDescriptor(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func CreateNewAcl(length int) (*Acl, error) {
	result := (*Acl)(unsafe.Pointer(&platform.AlignedBuffer(length, 4)[0]))
	err := InitializeAcl(result, uint32(length), ACL_REVISION)
	if err != nil {
		return nil, err
	}
	return result, nil
}


func GetUserObjectSecurity_Ex(obj syscall.Handle, sid uint32, desc []byte) (uint32, error) {
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

func GetUserObjectSecurity(obj syscall.Handle, sid uint32) ([]byte, error) {
	nLength, err := GetUserObjectSecurity_Ex(obj, sid, nil)
	if nLength == 0 {
		return nil, err
	}

	desc, err := CreateSecurityDescriptor(int(nLength))
	_, err = GetUserObjectSecurity_Ex(obj, sid, desc)
	if err != nil {
		return nil, err
	}
	return desc, err
}

type Acl struct{}

func GetSecurityDescriptorDacl(sid []byte) (present bool, acl *Acl, defaulted bool, err error) {
	var dPresent, dDefaulted uint32
	r1, _, e1 := procGetSecurityDescriptorDacl.Call(
		uintptr(unsafe.Pointer(&sid[0])),
		uintptr(unsafe.Pointer(&dPresent)),
		uintptr(unsafe.Pointer(&acl)),
		uintptr(unsafe.Pointer(&dDefaulted)))
	if dPresent != 0 {
		present = true
	}
	if dDefaulted != 0 {
		defaulted = true
	}
	if int(r1) == 0 {
		err = e1
	}
	return
}

func IsValidAcl(acl *Acl) bool {
	r1, _, _ := procIsValidAcl.Call(
		uintptr(unsafe.Pointer(acl)))
	if r1 == 0 {
		return false
	}
	return true
}

func GetAclInformation(acl *Acl, info unsafe.Pointer, length uint32, class uint32) error {
	r1, _, e1 := procGetAclInformation.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(info),
		uintptr(length),
		uintptr(class))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

type AclSizeInformation struct {
	AceCount uint32
	AclBytesInUse uint32
	AclBytesFree uint32
}

func GetAclSize(acl *Acl) (*AclSizeInformation, error) {
	var result AclSizeInformation
	err := GetAclInformation(acl, unsafe.Pointer(&result), uint32(unsafe.Sizeof(result)), 2)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func InitializeSecurityDescriptor(sd []byte) error {
	r1, _, e1 := procInitializeSecurityDescriptor.Call(
		uintptr(unsafe.Pointer(&sd[0])),
		SECURITY_DESCRIPTOR_REVISION)
	if int(r1) == 0 {
		return e1
	}
	return nil
}

func InitializeAcl(acl *Acl, length, revision uint32) error {
	r1, _, e1 := procInitializeAcl.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(length),
		uintptr(revision))
	if int(r1) == 0 {
		return e1
	}
	return nil
}

type AceHeader struct {
	AceType byte
	AceFlags byte
	AceSize uint16
}

type Ace struct {}

func AddAce(acl *Acl, revision, startIndex uint32, ace *Ace, size uint32) error {
	r1, _, e1 := procAddAce.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(revision),
		uintptr(startIndex),
		uintptr(unsafe.Pointer(ace)),
		uintptr(size))

	if int(r1) == 0 {
		return e1
	}
	return nil
}

func CopyAce(acl *Acl, ace *Ace) error {
	header := (*AceHeader)(unsafe.Pointer(ace))
	err := AddAce(acl, ACL_REVISION, 0xffffffff, ace, uint32(header.AceSize))
	return err
}
