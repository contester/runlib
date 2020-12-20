package service

import (
	"context"
	"time"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/storage"

	log "github.com/sirupsen/logrus"
)

func (s *Contester) GridfsCopy(request *contester_proto.CopyOperations, response *contester_proto.FileStats) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var sandbox *Sandbox
	var err error
	if request.GetSandboxId() != "" {
		sandbox, err = getSandboxById(s.Sandboxes, request.GetSandboxId())
		if err != nil {
			return err
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Entries = make([]*contester_proto.FileStat, 0, len(request.GetEntries()))
	for _, item := range request.GetEntries() {
		if item.GetLocalFileName() == "" || item.GetRemoteLocation() == "" {
			continue
		}

		resolved, _, err := resolvePath(s.Sandboxes, item.GetLocalFileName(), false)
		if err != nil {
			continue // TODO
		}

		stat, err := storage.FilerCopy(ctx, resolved, item.GetRemoteLocation(), item.GetUpload(),
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
