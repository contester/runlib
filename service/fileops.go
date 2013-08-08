package service

import (
	"code.google.com/p/goprotobuf/proto"
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"os"
	"path/filepath"
)

func statFile(name string, hash_it bool) (*contester_proto.FileStat, error) {
	result := &contester_proto.FileStat{}
	result.Name = &name
	info, err := os.Stat(name)
	if err != nil {
		// Handle ERROR_FILE_NOT_FOUND - return no error and nil instead of stat struct
		if tools.IsStatErrorFileNotFound(err) {
			return nil, nil
		}

		return nil, tools.NewError(err, "statFile", "os.Stat")
	}
	if info.IsDir() {
		result.IsDirectory = proto.Bool(true)
	} else {
		result.Size = proto.Uint64(uint64(info.Size()))
		if hash_it {
			result.Sha1Sum, err = tools.HashFile(name)
			if err != nil {
				return nil, tools.NewError(err, "statFile", "hashFile")
			}
		}
	}
	return result, nil
}

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.FileStats) error {
	ec := tools.NewContext("Stat")
	if request.SandboxId != nil {
		sandbox, err := getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return ec.NewError(err, "getSandboxById")
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Stats = make([]*contester_proto.FileStat, 0, len(request.Name))
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
			stat, err := statFile(name, request.GetCalculateSha1())
			if err != nil {
				return ec.NewError(err, "statFile")
			}
			if stat != nil {
				response.Stats = append(response.Stats, stat)
			}
		}

	}
	return nil
}
