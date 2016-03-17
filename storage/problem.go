package storage

import (
	"bufio"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ProblemStore interface {
	Backend

	GetNextRevision(id string) (int, error)
	SetManifest(manifest *ProblemManifest) error
	GetAllManifests() ([]ProblemManifest, error)
	Cleanup(latest int) error
}

type ProblemManifest struct {
	Key string `bson:"_id"`

	Id       string `json:"id"`
	Revision int    `json:"revision"`

	TestCount       int    `bson:"testCount",json:"test_count"`
	TimeLimitMicros int64  `bson:"timeLimitMicros",json:"time_limit_micros"`
	MemoryLimit     int64  `bson:"memoryLimit",json:"memory_limit"`
	Stdio           bool   `bson:"stdio",json:"stdio,omitempty"`
	TesterName      string `bson:"testerName",json:"tester_name"`
	Answers         []int  `bson:"answers",json:"answers,omitempty"`
	InteractorName  string `bson:"interactorName,omitempty",json:"interactor_name,omitempty"`
	CombinedHash    string `bson:"combinedHash,omitempty",json:"combined_hash,omitempty"`
}

func (s *ProblemManifest) GetGridPrefix() string {
	return idToGridPrefix(s.Id) + "/" + strconv.FormatInt(int64(s.Revision), 10) + "/"
}

func idToGridPrefix(id string) string {
	u, err := url.Parse(id)
	if err != nil {
		return ""
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		return "problem/polygon/" + u.Scheme + "/" + u.Host + u.Path
	}
	if u.Scheme == "direct" {
		return "problem/direct/" + u.Host + u.Path
	}
	return ""
}

func storeIfExists(backend Backend, filename, gridname string) error {
	if _, err := os.Stat(filename); err != nil {
		return err
	}

	_, err := backend.Copy(filename, gridname, true, "", "")
	if err != nil {
		return err
	}
	return nil
}

func readFirstLine(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	r := bufio.NewScanner(f)

	if r.Scan() {
		return strings.TrimSpace(r.Text()), nil
	}
	return "", nil
}
