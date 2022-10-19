package platform

type archDependentData struct{}

func (s *GlobalData) GetLoadLibraryW32() (uintptr, error) {
	return 0, nil
}
