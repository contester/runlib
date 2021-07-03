package platform

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/contester/runlib/win32"
	"golang.org/x/sys/windows"

	_ "embed"

	log "github.com/sirupsen/logrus"
)

type ContesterDesktop struct {
	WindowStation win32.Hwinsta
	Desktop       win32.Hdesk
	DesktopName   string
}

type GlobalData struct {
	mu sync.Mutex

	desktop        *ContesterDesktop
	loadLibraryW   uintptr
	loadLibraryW32 uintptr
}

type errNoGlobalDataT struct{}

func (errNoGlobalDataT) Error() string { return "" }

var errNoGlobalData = errNoGlobalDataT{}

func (s *GlobalData) GetDesktopName() (string, error) {
	if s == nil {
		return "", errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.desktop == nil {
		return "", errNoGlobalData
	}
	return s.desktop.DesktopName, nil
}

func (s *GlobalData) GetLoadLibraryW() (uintptr, error) {
	if s == nil {
		return 0, errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadLibraryW == 0 {
		return 0, errNoGlobalData
	}
	return s.loadLibraryW, nil
}

func (s *GlobalData) GetLoadLibraryW32() (uintptr, error) {
	if s == nil {
		return 0, errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadLibraryW32 == 0 {
		return 0, errNoGlobalData
	}
	return s.loadLibraryW32, nil
}

func threadIdName(prefix string) string {
	return prefix + strconv.FormatUint(uint64(windows.GetCurrentThreadId()), 10)
}

func createContesterDesktop() (result *ContesterDesktop, err error) {
	var desk win32.Hdesk
	var name string
	origWinsta, err := win32.GetProcessWindowStation()
	if err != nil {
		return
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origDesktop, err := win32.GetThreadDesktop(windows.GetCurrentThreadId())
	if err != nil {
		return nil, err
	}

	winsta, err := win32.CreateWindowStation(
		syscall.StringToUTF16Ptr(threadIdName("w")), 0, win32.MAXIMUM_ALLOWED, win32.MakeInheritSa())
	if err != nil {
		return nil, err
	}

	if err = win32.SetProcessWindowStation(winsta); err != nil {
		win32.CloseWindowStation(winsta)
		return
	}

	winstaName, err := win32.GetUserObjectName(syscall.Handle(winsta))
	if err == nil {
		shortName := threadIdName("c")

		desk, err = win32.CreateDesktop(
			syscall.StringToUTF16Ptr(shortName),
			nil, 0, 0, syscall.GENERIC_ALL, win32.MakeInheritSa())

		if err == nil {
			name = winstaName + "\\" + shortName
		}
	}

	win32.SetProcessWindowStation(origWinsta)
	win32.SetThreadDesktop(origDesktop)
	if err != nil {
		return
	}

	everyone, err := syscall.StringToSid("S-1-1-0")
	if err == nil {
		if err = win32.AddAceToWindowStation(winsta, everyone); err != nil {
			log.Error(err)
		}
		if err = win32.AddAceToDesktop(desk, everyone); err != nil {
			log.Error(err)
		}
	} else {
		err = os.NewSyscallError("StringToSid", err)
		log.Error(err)
	}

	return &ContesterDesktop{
		WindowStation: winsta,
		Desktop:       desk,
		DesktopName:   name,
	}, nil
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

func getLoadLibrary() (uintptr, error) {
	handle, err := win32.GetModuleHandle(syscall.StringToUTF16Ptr("kernel32"))
	if err != nil {
		return 0, err
	}
	addr, err := syscall.GetProcAddress(handle, "LoadLibraryW")
	if err != nil {
		return 0, os.NewSyscallError("GetProcAddress", err)
	}
	return addr, nil
}

type GlobalDataOptions struct {
	NeedDesktop     bool
	NeedLoadLibrary bool
}

func CreateGlobalData(opts GlobalDataOptions) (*GlobalData, error) {
	var err error
	var result GlobalData
	if opts.NeedDesktop {
		result.desktop, err = createContesterDesktop()
		if err != nil {
			return nil, err
		}
	}

	if opts.NeedLoadLibrary {
		result.loadLibraryW, err = getLoadLibrary()
		if err != nil {
			return nil, err
		}
		result.loadLibraryW32, err = getLoadLibrary32Bit()
		if err != nil {
			return nil, err
		}
	}
	return &result, nil
}
