package service

import (
	"os"
	"strings"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/subprocess"
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
	return os.Chmod(filename, s.Mode()|0100)
}
