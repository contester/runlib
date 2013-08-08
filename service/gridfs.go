package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"github.com/contester/runlib/mongotools"
)

func (s *Contester) GridfsCopy(request *contester_proto.CopyOperations, response *contester_proto.CopyOperationResults) error {
	var sandbox *Sandbox
	var err error
	if request.SandboxId != nil {
		sandbox, err = getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return tools.NewError(err, "GridfsGet", "getSandboxById")
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Entries = make([]*contester_proto.CopyOperationResult, 0, len(request.Entries))
	for _, item := range request.Entries {
		if item.LocalFileName == nil || item.RemoteLocation == nil {
			continue
		}

	    resolved, _, err := resolvePath(s.Sandboxes, item.GetLocalFileName(), false)
		if err != nil {
			continue // TODO
		}

		stringHash, err := mongotools.GridfsCopy(resolved, item.GetRemoteLocation(), s.Mfs, item.GetUpload(), item.GetChecksum(), item.GetModuleType())

		if !item.GetUpload() && sandbox != nil {
			err = sandbox.Own(resolved)
			if err != nil {
				continue
			}
		}

		response.Entries = append(response.Entries, &contester_proto.CopyOperationResult{
				LocalFileName: item.LocalFileName,
				Checksum: &stringHash,
			})
	}

	return nil
}
