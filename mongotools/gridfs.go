package mongotools

import (
	"github.com/contester/runlib/tools"
	"os"
	"io"
	"labix.org/v2/mgo"
)

func GridfsCopy(srcname, dstname string, mfs *mgo.GridFS, toGridfs bool) error {
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
