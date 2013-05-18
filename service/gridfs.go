package service

import (
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
	"io"
	"labix.org/v2/mgo"
	"os"
)

func gridfsCopy(srcname, dstname string, mfs *mgo.GridFS, toGridfs bool) error {
	var err error
	ec := tools.NewContext("gridfsCopy")

	var source io.ReadCloser
	var destination io.WriteCloser

	if toGridfs {
		source, err = os.Open(srcname)
	} else {
		source, err = mfs.Open(srcname)
	}
	if err != nil {
		return ec.NewError(err, "source.Open")
	}
	defer source.Close()

	if toGridfs {
		destination, err = mfs.Create(dstname)
	} else {
		destination, err = os.Create(dstname)
	}
	if err != nil {
		return ec.NewError(err, "destination.Open")
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return ec.NewError(err, "io.Copy")
	}

	if err = destination.Close(); err != nil {
		return ec.NewError(err, "destination.Close")
	}

	if err = source.Close(); err != nil {
		return ec.NewError(err, "source.Close")
	}

	return nil
}

func (s *Contester) GridfsGet(request *contester_proto.RepeatedNamePairEntries, response *contester_proto.RepeatedStringEntries) error {
	if request.SandboxId != nil {
		sandbox, err := getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return tools.NewComponentError(err, "GridfsGet", "getSandboxById")
		}
		sandbox.Mutex.RLock()
		defer sandbox.Mutex.RUnlock()
	}

	response.Entries = make([]string, 0, len(request.Entries))

	// TODO: add error reporting
	for _, item := range request.Entries {
		if item.Source == nil || item.Destination == nil {
			continue
		}
		resolved, _, err := resolvePath(s.Sandboxes, *item.Source, false)
		if err != nil {
			continue
		}
		err = gridfsCopy(resolved, *item.Destination, s.Mfs, true)
		if err != nil {
			continue
		}
		response.Entries = append(response.Entries, *item.Destination)
	}
	return nil
}

func (s *Contester) GridfsPut(request *contester_proto.RepeatedNamePairEntries, response *contester_proto.RepeatedStringEntries) error {
	ec := tools.NewContext("GridfsPut")
	var sandbox *Sandbox
	if request.SandboxId != nil {
		var err error
		sandbox, err = getSandboxById(s.Sandboxes, *request.SandboxId)
		if err != nil {
			return ec.NewError(err, "getSandboxById")
		}
		sandbox.Mutex.Lock()
		defer sandbox.Mutex.Unlock()
	}

	response.Entries = make([]string, 0, len(request.Entries))
	for _, item := range request.Entries {
		if item.Source == nil || item.Destination == nil {
			continue
		}
		resolved, _, err := resolvePath(s.Sandboxes, *item.Destination, true)
		if err != nil {
			return ec.NewError(err, "resolvePath")
		}
		err = gridfsCopy(*item.Source, resolved, s.Mfs, false)
		if err != nil {
			return ec.NewError(err, "gridfsCopy")
		}
		if sandbox != nil {
			err = sandbox.Own(resolved)
			if err != nil {
				return ec.NewError(err, "sandbox.Own")
			}
		}
		response.Entries = append(response.Entries, *item.Source)
	}
	return nil
}
