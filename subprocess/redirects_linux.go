// +build linux

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
