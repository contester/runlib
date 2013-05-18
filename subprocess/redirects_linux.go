package subprocess

import (
	"os"
)

func OpenFileForRedirect(name string, read bool) (*os.File, error) {
	if read {
		return os.Open(name)
	}
	return os.Create(name)
}

func ReaderDefault() (*os.File, error) {
	return os.Open("/dev/null")
}

func WriterDefault() (*os.File, error) {
	return os.Create("/dev/null")
}
