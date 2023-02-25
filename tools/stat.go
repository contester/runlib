package tools

import (
	"os"

	"github.com/contester/runlib/contester_proto"
	"github.com/juju/errors"
)

func StatFile(name string, hash_it bool) (*contester_proto.FileStat, error) {
	result := contester_proto.FileStat{
		Name: name,
	}
	info, err := os.Stat(name)
	if err != nil {
		// Handle ERROR_FILE_NOT_FOUND - return no error and nil instead of stat struct
		if IsStatErrorFileNotFound(err) {
			return nil, nil
		}

		return nil, errors.Annotatef(err, "os.Stat(%q)", name)
	}
	if info.IsDir() {
		result.IsDirectory = true
	} else {
		result.Size = uint64(info.Size())
		if hash_it {
			checksum, err := HashFileString(name)
			if err != nil {
				return nil, err
			}
			result.Checksum = checksum
		}
	}
	return &result, nil
}
