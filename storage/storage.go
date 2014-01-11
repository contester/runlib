package storage

import (
	"github.com/contester/runlib/contester_proto"
	"strings"
	"fmt"
)

type Backend interface {
	String() string
	Copy(localName, remoteName string, toRemote bool, checksum, moduleType string) (stat *contester_proto.FileStat, err error)
	Close()
}

// mongodb://...

type Storage struct {
	Default Backend
	Backends map[string]Backend
}

func NewStorage() *Storage {
	return &Storage{
		Backends: make(map[string]Backend),
	}
}

func NewBackend(url string) (Backend, error) {
	if strings.HasPrefix(url, "mongodb:") {
		return NewMongoDB(url)
	}
	return nil, fmt.Errorf("Can't parse storage url: %s", url)
}

func (s *Storage) SetDefault(url string) error {
	if s.Default != nil && s.Default.String() == url {
		return nil
	}
	backend, err := NewBackend(url)
	if err != nil {
		return err
	}
	s.Default = backend
	return nil
}
