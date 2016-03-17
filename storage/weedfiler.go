package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/tools"
)

var _ ProblemStore = &weedfilerStorage{}

type weedfilerStorage struct {
	URL string
	mu  sync.RWMutex
}

func NewWeed(url string) *weedfilerStorage {
	return &weedfilerStorage{
		URL: url,
	}
}

func (s *weedfilerStorage) upload(localName, remoteName string) error {
	ec := tools.ErrorContext("weedfiler.upload")
	local, err := os.Open(localName)
	if err != nil {
		return ec.NewError(err, "local.Open")
	}
	defer local.Close()

	req, err := http.NewRequest("PUT", s.URL+"fs/"+remoteName, local)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		resp.Body.Close()
	}
	return err
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

	if toRemote {
		err = s.upload(localName, remoteName)
	}

	return stat, err
}

func (s *weedfilerStorage) Cleanup(latest int) error {
	return nil
}

func (s *weedfilerStorage) Close() {
}

func (s *weedfilerStorage) GetAllManifests() ([]ProblemManifest, error) {
	resp, err := http.Get(s.URL + "problem/get/")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result []ProblemManifest
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func (s *weedfilerStorage) GetNextRevision(id string) (int, error) {
	resp, err := http.Get(s.URL + "problem/get/?id=" + url.QueryEscape(id))
	if err != nil {
		return 1, err
	}
	defer resp.Body.Close()
	var result []ProblemManifest
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return 1, err
	}
	if len(result) == 0 {
		return 1, nil
	}
	return result[0].Revision + 1, nil
}

func (s *weedfilerStorage) SetManifest(manifest *ProblemManifest) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	resp, err := http.Post(s.URL+"problem/set/", "application/octet-stream", bytes.NewReader(data))
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (s *weedfilerStorage) String() string {
	return s.URL
}
