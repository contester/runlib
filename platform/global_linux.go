package platform

type GlobalData struct{}

type ContesterDesktop struct {
}

type GlobalDataOptions struct {
	NeedDesktop, NeedLoadLibrary bool
}

func CreateGlobalData(opts GlobalDataOptions) (*GlobalData, error) {
	return nil, nil
}
