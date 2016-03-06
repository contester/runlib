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
	Cleanup(latest int) error
}

type ProblemManifest struct {
	Key string `bson:"_id"`

	Id       string
	Revision int

	TestCount       int    `bson:"testCount"`
	TimeLimitMicros int64  `bson:"timeLimitMicros"`
	MemoryLimit     int64  `bson:"memoryLimit"`
	Stdio           bool   `bson:"stdio"`
	TesterName      string `bson:"testerName"`
	Answers         []int  `bson:"answers"`
	InteractorName  string `bson:"interactorName,omitempty"`
	CombinedHash    string `bson:"combinedHash,omitempty"`
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
