package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/subprocess"
)

func (s *Contester) localPlatformSetup(sub *subprocess.Subprocess, request *contester_proto.LocalExecutionParameters) error {
	sub.Options.Environment = s.GData
	return nil
}

func chmodIfNeeded(filename string, sandbox *Sandbox) error {
	return nil
}
