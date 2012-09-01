package service

import (
	"runlib/contester_proto"
	"runlib/subprocess"
)

func (s *Contester) localPlatformSetup(sub *subprocess.Subprocess, request *contester_proto.LocalExecutionParameters) error {
	return nil
}

