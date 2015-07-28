package storage

import (
	"strings"
	"fmt"

	"github.com/contester/runlib/contester_proto"
	log "github.com/Sirupsen/logrus"
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
		log.Debug("New url %s is the same as the old %s", url, s.Default.String())
		return nil
	}
	backend, err := NewBackend(url)
	if err != nil {
		return err
	}
	if s.Default != nil {
		s.Default.Close()
	}
	s.Default = backend
	return nil
}
