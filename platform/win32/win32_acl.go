package win32

// +build windows,386

import (
	"runlib/tools"
	"syscall"
	"unsafe"
	l4g "code.google.com/p/log4go"
)

var (
	procGetUserObjectSecurity        = user32.NewProc("GetUserObjectSecurity")
	procSetUserObjectSecurity        = user32.NewProc("SetUserObjectSecurity")
	procGetSecurityDescriptorDacl    = advapi32.NewProc("GetSecurityDescriptorDacl")
	procSetSecurityDescriptorDacl    = advapi32.NewProc("SetSecurityDescriptorDacl")
	procIsValidAcl                   = advapi32.NewProc("IsValidAcl")
	procGetAclInformation            = advapi32.NewProc("GetAclInformation")
	procInitializeSecurityDescriptor = advapi32.NewProc("InitializeSecurityDescriptor")
	procInitializeAcl                = advapi32.NewProc("InitializeAcl")
	procAddAce                       = advapi32.NewProc("AddAce")
	procGetAce                       = advapi32.NewProc("GetAce")
	procAddAccessAllowedAce          = advapi32.NewProc("AddAccessAllowedAce")
	procAddAccessAllowedAceEx        = advapi32.NewProc("AddAccessAllowedAceEx")
)

const (
	DACL_SECURITY_INFORMATION    = 0x00000004
	SECURITY_DESCRIPTOR_REVISION = 1
	ACL_REVISION                 = 2

	DESKTOP_CREATEMENU       = 0x4
	DESKTOP_CREATEWINDOW     = 0x2
	DESKTOP_ENUMERATE        = 0x40
	DESKTOP_HOOKCONTROL      = 0x8
	DESKTOP_JOURNALPLAYBACK  = 0x20
	DESKTOP_JOURNALRECORD    = 0x10
	DESKTOP_READOBJECTS      = 0x1
	DESKTOP_SWITCHDESKTOP    = 0x100
	DESKTOP_WRITEOBJECTS     = 0x80
	STANDARD_RIGHTS_REQUIRED = 0x000F0000
	READ_CONTROL             = 0x00020000

	DESKTOP_ALL = (DESKTOP_CREATEMENU | DESKTOP_CREATEWINDOW | DESKTOP_ENUMERATE | DESKTOP_HOOKCONTROL |
		DESKTOP_JOURNALPLAYBACK | DESKTOP_JOURNALRECORD | DESKTOP_READOBJECTS | DESKTOP_SWITCHDESKTOP |
		DESKTOP_WRITEOBJECTS | READ_CONTROL)

	WINSTA_ALL_ACCESS = 0x37F
	WINSTA_ALL        = WINSTA_ALL_ACCESS | READ_CONTROL

	CONTAINER_INHERIT_ACE    = 2
	INHERIT_ONLY_ACE         = 8
	OBJECT_INHERIT_ACE       = 1
	NO_PROPAGATE_INHERIT_ACE = 4
)

func AddAceToDesktop(desk Hdesk, sid *syscall.SID) error {
	secDesc, err := GetUserObjectSecurity(syscall.Handle(desk), DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	_, acl, _, err := GetSecurityDescriptorDacl(secDesc)
	if err != nil {
		return err
	}

	newDesc, err := CreateSecurityDescriptor(2048)
	if err != nil {
		return err
	}
	newAcl, err := CreateNewAcl(1024)
	if err != nil {
		return err
	}
	err = CopyAllAce(newAcl, acl)
	if err != nil {
		return err
	}
	err = AddAccessAllowedAce(newAcl, ACL_REVISION, DESKTOP_ALL, sid)
	if err != nil {
		return err
	}
	err = SetSecurityDescriptorDacl(newDesc, true, newAcl, false)
	if err != nil {
		return err
	}
	err = SetUserObjectSecurity(syscall.Handle(desk), DACL_SECURITY_INFORMATION, newDesc)
	return err
}

func CopyAllAce(dest, source *Acl) error {
	if source == nil {
		return nil
	}
	aclSize, err := GetAclSize(source)
	if err != nil {
		return err
	}
	if aclSize == nil {
		return nil
	}
	for i := uint32(0); i < aclSize.AceCount; i++ {
		ace, err := GetAce(source, i)
		if err != nil {
			return err
		}
		err = CopyAce(dest, ace)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddAceToWindowStation(winsta Hwinsta, sid *syscall.SID) error {
	secDesc, err := GetUserObjectSecurity(syscall.Handle(winsta), DACL_SECURITY_INFORMATION)
	if err != nil {
		l4g.Error(err)
		return err
	}
	_, acl, _, err := GetSecurityDescriptorDacl(secDesc)
	if err != nil {
		l4g.Error(err)
		return err
	}

	newDesc, err := CreateSecurityDescriptor(2048)
	if err != nil {
		l4g.Error(err)
		return err
	}
	newAcl, err := CreateNewAcl(1024)
	if err != nil {
		l4g.Error(err)
		return err
	}
	err = CopyAllAce(newAcl, acl)
	if err != nil {
		l4g.Error(err)
		return err
	}
	err = AddAccessAllowedAceEx(newAcl, ACL_REVISION, CONTAINER_INHERIT_ACE|INHERIT_ONLY_ACE|OBJECT_INHERIT_ACE,
		syscall.GENERIC_ALL, sid)
	if err != nil {
		l4g.Error(err)
		return err
	}
	err = AddAccessAllowedAceEx(newAcl, ACL_REVISION, NO_PROPAGATE_INHERIT_ACE,
		WINSTA_ALL, sid)
	if err != nil {
		l4g.Error(err)
		return err
	}
	err = SetSecurityDescriptorDacl(newDesc, true, newAcl, false)
	if err != nil {
		l4g.Error(err)
		return err
	}
	err = SetUserObjectSecurity(syscall.Handle(winsta), DACL_SECURITY_INFORMATION, newDesc)

	return err
}

func CreateSecurityDescriptor(length int) ([]byte, error) {
	result := tools.AlignedBuffer(length, 4)
	err := InitializeSecurityDescriptor(result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func CreateNewAcl(length int) (*Acl, error) {
	result := (*Acl)(unsafe.Pointer(&tools.AlignedBuffer(length, 4)[0]))
	err := InitializeAcl(result, uint32(length), ACL_REVISION)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetUserObjectSecurity_Ex(obj syscall.Handle, sid uint32, desc []byte) (uint32, error) {
	var nLength uint32
	var nptr uintptr
	if desc != nil {
		nptr = uintptr(unsafe.Pointer(&desc[0]))
	}
	r1, _, e1 := procGetUserObjectSecurity.Call(
		uintptr(obj),
		uintptr(unsafe.Pointer(&sid)),
		nptr,
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

func SetUserObjectSecurity(obj syscall.Handle, sid uint32, desc []byte) error {
	r1, _, e1 := procSetUserObjectSecurity.Call(
		uintptr(obj),
		uintptr(unsafe.Pointer(&sid)),
		uintptr(unsafe.Pointer(&desc[0])))

	if int(r1) == 0 {
		return e1
	}
	return nil
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
	AceCount      uint32
	AclBytesInUse uint32
	AclBytesFree  uint32
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
	AceType  byte
	AceFlags byte
	AceSize  uint16
}

type Ace struct{}

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

func GetAce(acl *Acl, index uint32) (*Ace, error) {
	var result *Ace
	r1, _, e1 := procGetAce.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(index),
		uintptr(unsafe.Pointer(&result)))
	if int(r1) == 0 {
		return nil, e1
	}
	return result, nil
}

func CopyAce(acl *Acl, ace *Ace) error {
	header := (*AceHeader)(unsafe.Pointer(ace))
	err := AddAce(acl, ACL_REVISION, 0xffffffff, ace, uint32(header.AceSize))
	return err
}

func AddAccessAllowedAce(acl *Acl, revision, mask uint32, sid *syscall.SID) error {
	r1, _, e1 := procAddAccessAllowedAce.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(revision),
		uintptr(mask),
		uintptr(unsafe.Pointer(sid)))

	if int(r1) == 0 {
		return e1
	}
	return nil
}

func SetSecurityDescriptorDacl(sd []byte, present bool, acl *Acl, defaulted bool) error {
	var ppresent, pdefault uint32
	if present {
		ppresent = 1
	}
	if defaulted {
		pdefault = 1
	}

	r1, _, e1 := procSetSecurityDescriptorDacl.Call(
		uintptr(unsafe.Pointer(&sd[0])),
		uintptr(ppresent),
		uintptr(unsafe.Pointer(acl)),
		uintptr(pdefault))

	if int(r1) == 0 {
		return e1
	}
	return nil
}

func AddAccessAllowedAceEx(acl *Acl, revision, flags, mask uint32, sid *syscall.SID) error {
	r1, _, e1 := procAddAccessAllowedAceEx.Call(
		uintptr(unsafe.Pointer(acl)),
		uintptr(revision),
		uintptr(flags),
		uintptr(mask),
		uintptr(unsafe.Pointer(sid)))

	if int(r1) == 0 {
		return e1
	}
	return nil
}
