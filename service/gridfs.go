package service

import (
	"io"
	"os"
	"runlib/contester_proto"
	"labix.org/v2/mgo"
)

func gridfsCopy(srcname, dstname string, mfs *mgo.GridFS, toGridfs bool) error {
	var err error

	var source io.ReadCloser
	var destination io.WriteCloser

	if toGridfs {
		source, err = os.Open(srcname)
	} else {
		source, err = mfs.Open(srcname)
	}
	if err != nil {
		return err
	}
	defer source.Close()

	if toGridfs {
		destination, err = mfs.Create(dstname)
	} else {
		destination, err = os.Create(dstname)
	}
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	if err = destination.Close(); err != nil {
		return err
	}

	if err = source.Close(); err != nil {
		return err
	}

	return nil
}

func (s *Contester) GridfsGet(request *contester_proto.RepeatedNamePairEntries, response *contester_proto.RepeatedStringEntries) error {
	response.Entries = make([]string, 0, len(request.Entries))

	for _, item := range request.Entries {
		if item.Source == nil || item.Destination == nil {
			continue
		}
		resolved, err := resolvePath(s.Sandboxes, *item.Source, false)
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
	response.Entries = make([]string, 0, len(request.Entries))
	for _, item := range request.Entries {
		if item.Source == nil || item.Destination == nil {
			continue
		}
		resolved, err := resolvePath(s.Sandboxes, *item.Destination, true)
		if err != nil {
			return err
		}
		err = gridfsCopy(*item.Source, resolved, s.Mfs, false)
		if err != nil {
			return err
		}
		response.Entries = append(response.Entries, *item.Source)
	}
	return nil
}