package mongotools

import (
	"github.com/contester/runlib/tools"
	"os"
	"io"
	"labix.org/v2/mgo"
	"fmt"
	"compress/zlib"
)

type fileMetadata struct {
	Checksum string `bson:"checksum,omitempty"`
	ModuleType string `bson:"moduleType,omitempty"`
	CompressionType string `bson:"compressionType,omitempty"`
	OriginalSize uint64 `bson:"originalSize"`
}

func GridfsCopy(localName, remoteName string, mfs *mgo.GridFS, toGridfs bool, checksum, moduleType string) (string, error) {
	var err error
	ec := tools.NewContext("GridfsCopy")

	if toGridfs {
		calculatedChecksum, err := tools.HashFileString(localName)
		if err != nil {
			return "", ec.NewError(err, "local.CalculateChecksum")
		}

		if checksum != "" && calculatedChecksum != checksum {
			return "", ec.NewError(fmt.Errorf("Checksum mismatch, local %s != %s", calculatedChecksum, checksum))
		}

		checksum = calculatedChecksum
	}

	var local *os.File
	if toGridfs {
		local, err = os.Open(localName)
	} else {
		local, err = os.Create(localName)
	}

	if err != nil {
		return "", ec.NewError(err, "local.Open")
	}
	defer local.Close()

	var remote *mgo.GridFile
	if toGridfs {
		remote, err = mfs.Create(remoteName)
	} else {
		remote, err = mfs.Open(remoteName)
	}
	if err != nil {
		return "", ec.NewError(err, "remote.Open")
	}
	defer remote.Close()

	var source io.ReadCloser
	if toGridfs {
		source = local
	} else {
		source = remote
		var meta fileMetadata
		if err = remote.GetMeta(&meta); err != nil {
			return "", ec.NewError(err, "remote.GetMeta")
		}
		if meta.CompressionType == "ZLIB" {
			source, err = zlib.NewReader(source)
			if err != nil {
				return "", ec.NewError(err, "zlib.NewReader")
			}
		}
	}

	var destination io.WriteCloser
	if toGridfs {
		destination = zlib.NewWriter(remote)
	} else {
		destination = local
	}

	size, err := io.Copy(destination, source)
	if err != nil {
		return "", ec.NewError(err, "io.Copy")
	}

	if toGridfs {
		var meta fileMetadata
		meta.OriginalSize = uint64(size)
		meta.CompressionType = "ZLIB"
		meta.Checksum = checksum
		meta.ModuleType = moduleType

		remote.SetMeta(meta)
	}

	if err = destination.Close(); err != nil {
		return "", ec.NewError(err, "destination.Close")
	}

	if err = source.Close(); err != nil {
		return "", ec.NewError(err, "source.Close")
	}

	return checksum, nil
}
