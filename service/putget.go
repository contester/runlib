package service

import (
	"fmt"
	"os"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"google.golang.org/protobuf/proto"
)

func (s *Contester) Put(request *contester_proto.FileBlob, response *contester_proto.FileStat) error {
	resolved, sandbox, err := resolvePath(s.Sandboxes, request.GetName(), true)
	if err != nil {
		return err
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
			return fmt.Errorf("os.Create(%q): %w", resolved, err)
		}
		if !loop {
			break
		}
	}
	data, err := request.GetData().Bytes()
	if err != nil {
		destination.Close()
		return err
	}
	_, err = destination.Write(data)
	destination.Close()
	if err != nil {
		return fmt.Errorf("destination.Write(): %w", err)
	}
	if sandbox != nil {
		return sandbox.Own(resolved)
	}

	stat, err := tools.StatFile(resolved, true)
	if err != nil {
		return err
	}

	response.Reset()
	proto.Merge(response, stat)

	return nil
}

func (s *Contester) Get(request *contester_proto.GetRequest, response *contester_proto.FileBlob) error {
	resolved, sandbox, err := resolvePath(s.Sandboxes, request.GetName(), false)
	if err != nil {
		return err
	}

	if sandbox != nil {
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	source, err := os.Open(resolved)
	if err != nil {
		return fmt.Errorf("os.Open(%q): %w", resolved, err)
	}
	defer source.Close()

	response.Name = resolved
	response.Data, err = contester_proto.BlobFromStream(source)
	return err
}
