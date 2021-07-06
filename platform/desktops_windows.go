package platform

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/contester/runlib/win32"
	"golang.org/x/sys/windows"

	log "github.com/sirupsen/logrus"
)

type ContesterDesktop struct {
	WindowStation win32.Hwinsta
	Desktop       win32.Hdesk
	DesktopName   string
}

type GlobalData struct {
	mu sync.Mutex

	desktop    *ContesterDesktop
	desktopErr error

	loadLibraryW    uintptr
	loadLibraryWErr error

	archDependentData
}

type errNoGlobalDataT struct {
}

func (s errNoGlobalDataT) Error() string { return "no global data" }

var errNoGlobalData = errNoGlobalDataT{}

func (s *GlobalData) onceInitDesktop() {
	s.desktop, s.desktopErr = createContesterDesktop()
}

func (s *GlobalData) GetDesktopName() (string, error) {
	if s == nil {
		return "", errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.desktop == nil && s.desktopErr == nil {
		s.onceInitDesktop()
	}

	if s.desktopErr != nil {
		return "", s.desktopErr
	}

	return s.desktop.DesktopName, nil
}

func (s *GlobalData) onceInitLibraryW() {
	s.loadLibraryW, s.loadLibraryWErr = getLoadLibrary()
}

func (s *GlobalData) GetLoadLibraryW() (uintptr, error) {
	if s == nil {
		return 0, errNoGlobalData
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loadLibraryWErr == nil && s.loadLibraryW == 0 {
		s.onceInitLibraryW()
	}

	if s.loadLibraryWErr != nil {
		return 0, s.loadLibraryWErr
	}
	return s.loadLibraryW, nil
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
			shortName, 0, syscall.GENERIC_ALL, win32.MakeInheritSa())

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

func getLoadLibrary() (uintptr, error) {
	handle, err := win32.GetModuleHandle("kernel32")
	if err != nil {
		return 0, err
	}
	addr, err := syscall.GetProcAddress(handle, "LoadLibraryW")
	if err != nil {
		return 0, os.NewSyscallError("GetProcAddress", err)
	}
	return addr, nil
}

func CreateGlobalData(needDesktop bool) (*GlobalData, error) {
	var result GlobalData
	if needDesktop {
		result.onceInitDesktop()
		if result.desktopErr != nil {
			return nil, result.desktopErr
		}
	}
	return &result, nil
}
