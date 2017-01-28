package service

import (
	log "github.com/Sirupsen/logrus"
	"github.com/contester/runlib/contester_proto"
	"github.com/juju/errors"
)

func (s *Contester) GridfsCopy(request *contester_proto.CopyOperations, response *contester_proto.FileStats) error {
	var sandbox *Sandbox
	var err error
	if request.SandboxId != nil {
		sandbox, err = getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return err
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Storage == nil {
		return errors.BadRequestf("can't gridfs.Copy if storage isn't set")
	}

	response.Entries = make([]*contester_proto.FileStat, 0, len(request.Entries))
	for _, item := range request.Entries {
		if item.LocalFileName == nil || item.RemoteLocation == nil {
			continue
		}

		resolved, _, err := resolvePath(s.Sandboxes, item.GetLocalFileName(), false)
		if err != nil {
			continue // TODO
		}

		stat, err := s.Storage.Copy(resolved, item.GetRemoteLocation(), item.GetUpload(),
			item.GetChecksum(), item.GetModuleType(), item.GetAuthorizationToken())

		if err != nil {
			log.Errorf("gridfs copy error: %+v", err)
			continue
		}

		if !item.GetUpload() && sandbox != nil {
			err = sandbox.Own(resolved)
			if err != nil {
				log.Errorf("sandbox.Own error: %+v", err)
				continue
			}
		}

		if stat != nil {
			response.Entries = append(response.Entries, stat)
		}
	}

	return nil
}
