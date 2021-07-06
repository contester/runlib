package subprocess

type archDependentPlatformData struct{}

func (s *SubprocessData) initArchDependentData(sub *Subprocess) error { return nil }

type PlatformEnvironment interface {
	GetDesktopName() (string, error)
	GetLoadLibraryW() (uintptr, error)
}

func (s *SubprocessData) getLoadLibraryW(env PlatformEnvironment) (uintptr, error) {
	return env.GetLoadLibraryW()
}
