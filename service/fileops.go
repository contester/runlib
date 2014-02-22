package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"path/filepath"
)

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.FileStats) error {
	ec := tools.ErrorContext("Stat")
	if request.SandboxId != nil {
		sandbox, err := getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return ec.NewError(err, "getSandboxById")
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Entries = make([]*contester_proto.FileStat, 0, len(request.Name))
	for _, name := range request.Name {
		resolved, _, err := resolvePath(s.Sandboxes, name, false)
		if err != nil {
			return ec.NewError(err, "resolvePath")
		}
		var expanded []string
		if request.GetExpand() {
			expanded, err = filepath.Glob(resolved)
			if err != nil {
				return ec.NewError(err, "filepath.Glob")
			}
		} else {
			expanded = []string{resolved}
		}

		for _, name := range expanded {
			stat, err := tools.StatFile(name, request.GetCalculateChecksum())
			if err != nil {
				return ec.NewError(err, "statFile")
			}
			if stat != nil {
				response.Entries = append(response.Entries, stat)
			}
		}

	}
	return nil
}
