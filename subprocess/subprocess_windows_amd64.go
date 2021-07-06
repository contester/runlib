package subprocess

import "github.com/contester/runlib/win32"

type archDependentPlatformData struct {
	use32BitLoadLibrary bool
}

func (s *SubprocessData) initArchDependentData(sub *Subprocess) error {
	if len(sub.Options.InjectDLL) != 0 {
		binaryType, err := win32.GetBinaryType(getImageName(sub))
		if err != nil {
			return err
		}
		s.platformData.use32BitLoadLibrary = binaryType == win32.SCS_32BIT_BINARY
	}
	return nil
}

type PlatformEnvironment interface {
	GetDesktopName() (string, error)
	GetLoadLibraryW() (uintptr, error)
	GetLoadLibraryW32() (uintptr, error)
}

func (s *SubprocessData) getLoadLibraryW(env PlatformEnvironment) (uintptr, error) {
	if s.platformData.use32BitLoadLibrary {
		return env.GetLoadLibraryW32()
	}
	return env.GetLoadLibraryW()
}
