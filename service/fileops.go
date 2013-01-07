package service

import (
	"code.google.com/p/goprotobuf/proto"
	l4g "code.google.com/p/log4go"
	"os"
	"path/filepath"
	"runlib/contester_proto"
	"crypto/sha1"
	"io"
)

func statFile(name string) (*contester_proto.FileStat, error) {
	result := &contester_proto.FileStat{}
	result.Name = &name
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		result.IsDirectory = proto.Bool(true)
	} else {
		result.Size = proto.Uint64(uint64(info.Size()))
	}
	return result, nil
}

func hashFile(name string) ([]byte, error) {
	source, err := os.Open(name)
	if err != nil {
		return nil, NewServiceError("source.Open", err)
	}
	defer source.Close()

	destination := sha1.New()

	_, err = io.Copy(destination, source)
	if err != nil {
		return nil, NewServiceError("io.Copy", err)
	}
	if err = source.Close(); err != nil {
		return nil, NewServiceError("source.Close", err)
	}

	return destination.Sum(nil), nil
}

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.FileStats) error {
	if request.SandboxId != nil {
		sandbox, err := getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			l4g.Error(err)
			return err
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Stats = make([]*contester_proto.FileStat, 0, len(request.Name))
	for _, name := range request.Name {
		resolved, _, err := resolvePath(s.Sandboxes, name, false)
		if err != nil {
			l4g.Error(err)
			continue
		}
		var expanded []string
		if request.Expand != nil && *request.Expand {
			expanded, err = filepath.Glob(resolved)
			if err != nil {
				l4g.Error("Expanding", resolved, err)
				continue
			}
		} else {
			expanded = []string{resolved}
		}

		for _, name := range expanded {
			stat, _ := statFile(name)
			if stat != nil {
				response.Stats = append(response.Stats, stat)
			}
		}

	}
	return nil
}
