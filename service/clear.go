package service

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/contester/runlib/contester_proto"

	log "github.com/sirupsen/logrus"
)

func tryClearPath(path string) (bool, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("ReadDir(%q): %w", path, err)
	}

	if len(files) == 0 {
		return false, nil
	}

	for _, info := range files {
		if info.Name() == "." || info.Name() == ".." {
			continue
		}
		fullpath := filepath.Join(path, info.Name())
		err = os.RemoveAll(fullpath)
		if err != nil {
			return true, fmt.Errorf("os.RemoveAll(%q): %w", fullpath, err)
		}
	}
	return true, nil
}

func (s *Contester) Clear(request *contester_proto.ClearSandboxRequest, response *contester_proto.EmptyMessage) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	sandbox.Mutex.Lock()
	defer sandbox.Mutex.Unlock()

	repeat := true

	for retries := 10; retries > 0 && repeat; retries-- {
		repeat, err = tryClearPath(sandbox.Path)
		if repeat && err != nil {
			log.Error(err)
			time.Sleep(time.Second / 2)
		}
	}

	if err != nil {
		return err
	}
	return nil
}
