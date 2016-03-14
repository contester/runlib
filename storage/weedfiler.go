package storage

/*
import (
	"sync"
	"github.com/contester/runlib/contester_proto"
"github.com/contester/runlib/tools"
	"fmt"
	"os"
)

type weedfilerStorage struct {
	URL string
	mu sync.RWMutex
}


func (s *weedfilerStorage) Copy(localName, remoteName string, toRemote bool, checksum, moduleType string) (stat *contester_proto.FileStat, err error) {
	ec := tools.ErrorContext("mongodb.Copy")

	if toRemote {
		stat, err = tools.StatFile(localName, true)
		if err != nil {
			err = ec.NewError(err, "local.CalculateChecksum")
		}
		// If file doesn't exist then stat == nil.
		if err != nil || stat == nil {
			return
		}

		if checksum != "" && *stat.Checksum != checksum {
			return nil, ec.NewError(fmt.Errorf("Checksum mismatch, local %s != %s", stat.Checksum, checksum))
		}

		checksum = *stat.Checksum
	}

	var local *os.File
	if toRemote {
		local, err = os.Open(localName)
	} else {
		local, err = os.Create(localName)
	}

	if err != nil {
		return nil, ec.NewError(err, "local.Open")
	}
	defer local.Close()

	s.mu.RLock()
	defer s.mu.RUnlock()
	gridfs := s.gridfs()

	var remote *mgo.GridFile
	if toRemote {
		// Remove all files with the same remoteName.
		if err = gridfs.Remove(remoteName); err != nil {
			return nil, ec.NewError(err, "remote.Remove")
		}
		remote, err = gridfs.Create(remoteName)
	} else {
		remote, err = gridfs.Open(remoteName)
	}
	if err != nil {
		return nil, ec.NewError(err, "remote.Open")
	}
	defer remote.Close()

	var source io.ReadCloser
	if toRemote {
		source = local
	} else {
		source = remote
		var meta fileMetadata
		if err = remote.GetMeta(&meta); err != nil {
			return nil, ec.NewError(err, "remote.GetMeta")
		}
		if meta.CompressionType == "ZLIB" {
			source, err = zlib.NewReader(source)
			if err != nil {
				return nil, ec.NewError(err, "zlib.NewReader")
			}
		}
	}

	var destination io.WriteCloser
	if toRemote {
		destination = zlib.NewWriter(remote)
	} else {
		destination = local
	}

	size, err := io.Copy(destination, source)
	if err != nil {
		return nil, ec.NewError(err, "io.Copy")
	}

	if toRemote {
		var meta fileMetadata
		meta.OriginalSize = uint64(size)
		meta.CompressionType = "ZLIB"
		meta.Checksum = *stat.Checksum
		meta.ModuleType = moduleType

		remote.SetMeta(meta)
	}

	if err = destination.Close(); err != nil {
		return nil, ec.NewError(err, "destination.Close")
	}

	if err = source.Close(); err != nil {
		return nil, ec.NewError(err, "source.Close")
	}

	if !toRemote {
		stat, err = tools.StatFile(localName, true)
		if err != nil {
			return nil, ec.NewError(err, "StatFile")
		}
	}

	return stat, nil
}
*/