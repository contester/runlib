package service

import (
	"runlib/contester_proto"
	"runlib/subprocess"
)

func (s *Contester) localPlatformSetup(sub *subprocess.Subprocess, request *contester_proto.LocalExecutionParameters) error {
	if sub.Login != nil && !sub.NoJob {
		sub.Options.Desktop = s.GData.Desktop.DesktopName
	}
	return nil
}

