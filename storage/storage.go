package storage

import (
	"fmt"
	"strings"

	"github.com/contester/runlib/contester_proto"
)

type Backend interface {
	String() string
	Copy(localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error)
	Close()
}

// mongodb://...

type statelessBackend struct{}

var statelessBackendSingleton statelessBackend

func (s statelessBackend) String() string {
	return "Stateless"
}

func (s statelessBackend) Close() {}

func (s statelessBackend) Copy(localName, remoteName string, toRemote bool, checksum, moduleType, authToken string) (stat *contester_proto.FileStat, err error) {
	if fr := isFilerRemote(remoteName); fr != "" {
		return filerCopy(localName, fr, toRemote, checksum, moduleType, authToken)
	}
	return nil, fmt.Errorf("can't use stateless backend")
}

func NewBackend(url string) (Backend, error) {
	if strings.HasPrefix(url, "mongodb:") {
		return NewMongoDB(url)
	}
	if strings.HasPrefix(url, "http:") {
		return NewWeed(url), nil
	}
	return statelessBackendSingleton, nil
}
