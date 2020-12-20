package storage

import (
	"context"
	"net/url"
	"strconv"
)

type ProblemStore interface {
	Backend

	GetNextRevision(ctx context.Context, id string) (int, error)
	SetManifest(ctx context.Context, manifest *ProblemManifest) error
	GetAllManifests(ctx context.Context) ([]ProblemManifest, error)
}

type ProblemManifest struct {
	Key string `bson:"_id"`

	Id       string `json:"id"`
	Revision int    `json:"revision"`

	TestCount       int    `bson:"testCount" json:"testCount"`
	TimeLimitMicros int64  `bson:"timeLimitMicros" json:"timeLimitMicros"`
	MemoryLimit     int64  `bson:"memoryLimit" json:"memoryLimit"`
	Stdio           bool   `bson:"stdio" json:"stdio,omitempty"`
	TesterName      string `bson:"testerName" json:"testerName"`
	Answers         []int  `bson:"answers" json:"answers,omitempty"`
	InteractorName  string `bson:"interactorName,omitempty" json:"interactorName,omitempty"`
	CombinedHash    string `bson:"combinedHash,omitempty" json:"combinedHash,omitempty"`
}

func (s *ProblemManifest) GetGridPrefix() string {
	return IdToGridPrefix(s.Id) + "/" + strconv.FormatInt(int64(s.Revision), 10) + "/"
}

func IdToGridPrefix(id string) string {
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
