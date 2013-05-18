package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"os"
)

func (s *Contester) Put(request *contester_proto.FileBlob, response *contester_proto.EmptyMessage) error {
	ec := tools.NewContext("Put")
	if request.Data != nil {
		contester_proto.AddBlob(request.Data)
	}

	resolved, sandbox, err := resolvePath(s.Sandboxes, *request.Name, true)
	if err != nil {
		return ec.NewError(err, "resolvePath")
	}

	if sandbox != nil {
		sandbox.Mutex.Lock()
		defer sandbox.Mutex.Unlock()
	}

	var destination *os.File

	for {
		destination, err = os.Create(resolved)
		loop, err := OnOsCreateError(err)

		if err != nil {
			return ec.NewError(err, "os.Create")
		}
		if !loop {
			break
		}
	}
	data, err := request.Data.Bytes()
	if err != nil {
		return ec.NewError(err, "request.Data.Bytes")
	}
	_, err = destination.Write(data)
	if err != nil {
		return ec.NewError(err, "destination.Write")
	}
	destination.Close()
	if sandbox != nil {
		return sandbox.Own(resolved)
	}

	_, err = hashFile(resolved)
	if err != nil {
		return ec.NewError(err, "hashFile")
	}

	return nil
}

func (s *Contester) Get(request *contester_proto.GetRequest, response *contester_proto.FileBlob) error {
	ec := tools.NewContext("Get")
	resolved, sandbox, err := resolvePath(s.Sandboxes, *request.Name, false)
	if err != nil {
		return ec.NewError(err, "resolvePath")
	}

	if sandbox != nil {
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	source, err := os.Open(resolved)
	if err != nil {
		return ec.NewError(err, "os.Open")
	}
	defer source.Close()

	response.Name = &resolved
	response.Data, err = contester_proto.BlobFromStream(source)
	return err
}
