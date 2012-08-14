package service

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type Sandbox struct {
	Path               string
	Username, Password *string
}

type SandboxPair struct {
	Compile, Run Sandbox
}

func getSandboxById(s []SandboxPair, id string) (*Sandbox, error) {
	parts := strings.Split(id, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("Malformed sandbox ID %s", id)
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("Can't parse non-int sandbox index %s", parts[0])
	}

	if index < 0 || index >= len(s) {
		return nil, fmt.Errorf("Sandbox index %d is out of range (max=%d)", index, len(s))
	}

	switch strings.ToUpper(parts[1]) {
	case "C":
		return &s[index].Compile, nil
	case "R":
		return &s[index].Run, nil
	}
	return nil, fmt.Errorf("Sandbox variant %s is unknown", parts[1])
}

func getSandboxByPath(s []SandboxPair, id string) (*Sandbox, error) {
	cleanid := filepath.Clean(id)
	for _, v := range s {
		switch {
		case strings.HasPrefix(cleanid, v.Compile.Path):
			return &v.Compile, nil
		case strings.HasPrefix(cleanid, v.Run.Path):
			return &v.Run, nil
		}
	}
	return nil, fmt.Errorf("No sandbox corresponds to path %s", cleanid)
}
