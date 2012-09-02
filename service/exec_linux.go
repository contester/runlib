package service

import (
	"runlib/contester_proto"
	"runlib/subprocess"
	"strings"
	"os"
)

func (s *Contester) localPlatformSetup(sub *subprocess.Subprocess, request *contester_proto.LocalExecutionParameters) error {
	return nil
}

func chmodIfNeeded(filename string, sandbox *Sandbox) error {
	if !strings.HasPrefix(filename, sandbox.Path) {
		return nil
	}
	s, err := os.Stat(filename)
	if err != nil {
		return err
	}
	return os.Chmod(filename, s.Mode() | 0100)
}
