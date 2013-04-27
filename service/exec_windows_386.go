package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/subprocess"
)

func (s *Contester) localPlatformSetup(sub *subprocess.Subprocess, request *contester_proto.LocalExecutionParameters) error {
	if sub.Login != nil && !sub.NoJob {
		sub.Options.Desktop = s.GData.Desktop.DesktopName
	}
	return nil
}

func chmodIfNeeded(filename string, sandbox *Sandbox) error {
	return nil
}
