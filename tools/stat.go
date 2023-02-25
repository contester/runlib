package tools

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/contester/runlib/contester_proto"
)

func StatFile(name string, hash_it bool) (*contester_proto.FileStat, error) {
	result := contester_proto.FileStat{
		Name: name,
	}
	info, err := os.Stat(name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, fmt.Errorf("os.Stat(%q): %w", name, err)
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
