package service

import (
	"fmt"
	"path/filepath"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
)

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.FileStats) error {
	if request.GetSandboxId() != "" {
		sandbox, err := getSandboxById(s.Sandboxes, request.GetSandboxId())
		if err != nil {
			return err
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Entries = make([]*contester_proto.FileStat, 0, len(request.Name))
	for _, name := range request.Name {
		resolved, _, err := resolvePath(s.Sandboxes, name, false)
		if err != nil {
			return err
		}
		var expanded []string
		if request.GetExpand() {
			expanded, err = filepath.Glob(resolved)
			if err != nil {
				return fmt.Errorf("filepath.Glob(%q): %w", resolved, err)
			}
		} else {
			expanded = []string{resolved}
		}

		for _, name := range expanded {
			stat, err := tools.StatFile(name, request.GetCalculateChecksum())
			if err != nil {
				return err
			}
			if stat != nil {
				response.Entries = append(response.Entries, stat)
			}
		}

	}
	return nil
}
