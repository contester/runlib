package service

import (
	"code.google.com/p/goprotobuf/proto"
	"os"
	"syscall"
	"path/filepath"
	"runlib/contester_proto"
	"crypto/sha1"
	"io"
)

func statFile(name string, hash_it bool) (*contester_proto.FileStat, error) {
	result := &contester_proto.FileStat{}
	result.Name = &name
	info, err := os.Stat(name)
	if err != nil {
		// Handle ERROR_PATH_NOT_FOUND - return no error and nil instead of stat struct
		if path_err, ok := err.(*os.PathError); ok {
			if errno, ok := path_err.Err.(syscall.Errno); ok && errno == syscall.Errno(syscall.ERROR_PATH_NOT_FOUND) {
				return nil, nil
			}
		}

		return nil, NewServiceError("os.Stat", err)
	}
	if info.IsDir() {
		result.IsDirectory = proto.Bool(true)
	} else {
		result.Size = proto.Uint64(uint64(info.Size()))
		if hash_it {
			result.Sha1Sum, err = hashFile(name)
			if err != nil {
				return nil, NewServiceError("hashFile", err)
			}
		}
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
			return NewServiceError("getSandboxById", err)
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Stats = make([]*contester_proto.FileStat, 0, len(request.Name))
	for _, name := range request.Name {
		resolved, _, err := resolvePath(s.Sandboxes, name, false)
		if err != nil {
			return NewServiceError("resolvePath", err)
		}
		var expanded []string
		if request.GetExpand() {
			expanded, err = filepath.Glob(resolved)
			if err != nil {
				return NewServiceError("filepath.Glob", err)
			}
		} else {
			expanded = []string{resolved}
		}

		for _, name := range expanded {
			stat, err := statFile(name, request.GetCalculateSha1())
			if err != nil {
				return NewServiceError("statFile", err)
			}
			if stat != nil {
				response.Stats = append(response.Stats, stat)
			}
		}

	}
	return nil
}
