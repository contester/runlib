package storage

import (
	"fmt"
	"strings"

	"github.com/contester/runlib/contester_proto"
)

type Backend interface {
	String() string
	Copy(localName, remoteName string, toRemote bool, checksum, moduleType string) (stat *contester_proto.FileStat, err error)
	Close()
}

// mongodb://...

func NewBackend(url string) (Backend, error) {
	if strings.HasPrefix(url, "mongodb:") {
		return NewMongoDB(url)
	}
	return nil, fmt.Errorf("Can't parse storage url: %s", url)
}
