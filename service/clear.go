package service

import (
	l4g "code.google.com/p/log4go"
	"io/ioutil"
	"os"
	"path/filepath"
	"runlib/contester_proto"
)

func (s *Contester) Clear(request *contester_proto.ClearSandboxRequest, response *contester_proto.EmptyMessage) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	sandbox.Mutex.Lock()
	defer sandbox.Mutex.Unlock()

	path := sandbox.Path
	files, err := ioutil.ReadDir(path)
	if err != nil {
		l4g.Error(err)
		return err
	}

	for _, info := range files {
		if info.Name() == "." || info.Name() == ".." {
			continue
		}
		fullpath := filepath.Join(path, info.Name())
		err = os.RemoveAll(fullpath)
		if err != nil {
			l4g.Error(err)
			return err
		}
	}
	return nil
}
