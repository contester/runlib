package service

import (
	l4g "code.google.com/p/log4go"
	"os"
	"runlib/contester_proto"
	"syscall"
)

func (s *Contester) Put(request *contester_proto.FileBlob, response *contester_proto.EmptyMessage) error {
	if request.Data != nil {
		contester_proto.AddBlob(request.Data)
	}

	resolved, sandbox, err := resolvePath(s.Sandboxes, *request.Name, true)
	if err != nil {
		l4g.Error(err)
		return err
	}

	if sandbox != nil {
		sandbox.Mutex.Lock()
		defer sandbox.Mutex.Unlock()
	}

	var destination *os.File

	for {
		destination, err = os.Create(resolved)
		if err != nil {
			l4g.Error(err)
			if err == syscall.ERROR_ACCESS_DENIED {
				continue
			}
			return err
		}
		break
	}
	data, err := request.Data.Bytes()
	if err != nil {
		l4g.Error(err)
		return err
	}
	_, err = destination.Write(data)
	if err != nil {
		l4g.Error(err)
		return err
	}
	destination.Close()

	return nil
}

func (s *Contester) Get(request *contester_proto.GetRequest, response *contester_proto.FileBlob) error {
	resolved, sandbox, err := resolvePath(s.Sandboxes, *request.Name, false)
	if err != nil {
		return err
		l4g.Error(err)
	}

	if sandbox != nil {
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	source, err := os.Open(resolved)
	if err != nil {
		return err
		l4g.Error(err)
	}
	defer source.Close()

	response.Name = &resolved
	response.Data, err = contester_proto.BlobFromStream(source)
	return err
}
