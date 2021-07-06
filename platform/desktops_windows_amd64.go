package platform

import (
	"os"
	"os/exec"
	"strconv"

	_ "embed"
)

type archDependentData struct {
	loadLibraryW32    uintptr
	loadLibraryW32Err error
}

//go:embed Detect32BitEntryPoint.exe.embed
var detect32BitEntryPointBinary []byte

func getLoadLibrary32Bit() (uintptr, error) {
	tfile, err := os.CreateTemp("", "detect32bit.*.exe")
	if err != nil {
		return 0, err
	}
	fname := tfile.Name()
	defer os.Remove(fname)
	_, err = tfile.Write(detect32BitEntryPointBinary)
	if err != nil {
		tfile.Close()
		return 0, err
	}
	err = tfile.Close()
	if err != nil {
		return 0, err
	}

	cmd := exec.Command(fname)
	txt, err := cmd.CombinedOutput()
	if err != nil {
		return 0, err
	}
	cval, err := strconv.ParseInt(string(txt), 10, 64)
	if err != nil {
		return 0, err
	}
	return uintptr(cval), nil
}

func (s *GlobalData) onceInitLibraryW32() {
	s.loadLibraryW32, s.loadLibraryW32Err = getLoadLibrary32Bit()
}

func (s *GlobalData) GetLoadLibraryW32() (uintptr, error) {
	if s == nil {
		return 0, errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loadLibraryW32Err == nil && s.loadLibraryW32 == 0 {
		s.onceInitLibraryW32()
	}

	if s.loadLibraryW32Err != nil {
		return 0, s.loadLibraryW32Err
	}
	return s.loadLibraryW32, nil
}
