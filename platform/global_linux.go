package platform

type GlobalData struct{}

type ContesterDesktop struct {
}

func CreateGlobalData(opts bool) (*GlobalData, error) {
	return nil, nil
}

func (s *GlobalData) GetLoadLibraryW32() (uintptr, error) {
	return 0, nil
}
