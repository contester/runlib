package win32

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Error mode flags passed to SetErrorMode and SetProcessErrorMode.
const (
	SEM_FAILCRITICALERRORS     = windows.SEM_FAILCRITICALERRORS
	SEM_NOGPFAULTERRORBOX      = windows.SEM_NOGPFAULTERRORBOX
	SEM_NOALIGNMENTFAULTEXCEPT = windows.SEM_NOALIGNMENTFAULTEXCEPT
	SEM_NOOPENFILEERRORBOX     = windows.SEM_NOOPENFILEERRORBOX

	// SuppressErrorDialogsMask suppresses critical-error, GPF/WER crash, and
	// missing-file modal error dialogs in a process and its children.
	SuppressErrorDialogsMask = SEM_FAILCRITICALERRORS | SEM_NOGPFAULTERRORBOX | SEM_NOOPENFILEERRORBOX
)

// win32ToNtHardErrorMode converts a Win32 SetErrorMode bitmask into the value
// expected by NtSetInformationProcess(ProcessDefaultHardErrorMode).
//
// In EPROCESS.DefaultHardErrorProcessing, bit 0 (SEM_FAILCRITICALERRORS) is
// inverted relative to Win32: 0 suppresses critical-error popups and 1 enables
// them, matching kernel32!SetErrorMode.
func win32ToNtHardErrorMode(mode uint32) uint32 {
	return mode ^ SEM_FAILCRITICALERRORS
}

// SetProcessErrorMode sets the hard-error mode of another process (such as a
// suspended child) before it runs via NtSetInformationProcess.
func SetProcessErrorMode(process syscall.Handle, mode uint32) error {
	ntMode := win32ToNtHardErrorMode(mode)
	if err := windows.NtSetInformationProcess(
		windows.Handle(process),
		windows.ProcessDefaultHardErrorMode,
		unsafe.Pointer(&ntMode),
		uint32(unsafe.Sizeof(ntMode)),
	); err != nil {
		return fmt.Errorf("NtSetInformationProcess(ProcessDefaultHardErrorMode, 0x%x): %w", mode, err)
	}
	return nil
}
