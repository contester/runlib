package service

import (
	l4g "code.google.com/p/log4go"
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func tryClearPath(path string) (bool, error) {
	ec := tools.ErrorContext("tryClearPath")
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, ec.NewError(err, "ioutil.ReadDir")
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
			return true, ec.NewError(err, "os.RemoveAll")
		}
	}
	return true, nil
}

func (s *Contester) Clear(request *contester_proto.ClearSandboxRequest, response *contester_proto.EmptyMessage) error {
	ec := tools.ErrorContext("Clear")
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return ec.NewError(err, "getSandboxById")
	}

	sandbox.Mutex.Lock()
	defer sandbox.Mutex.Unlock()

	repeat := true

	for retries := 10; retries > 0 && repeat; retries-- {
		repeat, err = tryClearPath(sandbox.Path)
		if repeat && err != nil {
			l4g.Error(err)
			time.Sleep(time.Second / 2)
		}
	}

	if err != nil {
		return ec.NewError(err, "tryClearPath")
	}
	return nil
}
