package service

import (
	"os"
)

func (s *Sandbox) Own(filename string) error {
	return os.Chown(filename, s.Login.Uid, 0)
}
