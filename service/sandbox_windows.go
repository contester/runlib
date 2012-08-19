package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Sandbox struct {
	Path               string
	Username, Password *string
	Mutex sync.Mutex
}

type SandboxPair struct {
	Compile, Run Sandbox
}

func getSandboxById(s []SandboxPair, id string) (*Sandbox, error) {
	if len(id) < 4 || id[0] != '%' {
		return nil, fmt.Errorf("Malformed sandbox ID %s", id)
	}
	parts := strings.Split(id[1:], ".")
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

func resolvePath(s []SandboxPair, source string, restricted bool) (string, error) {
	if len(source) < 1 {
		return source, fmt.Errorf("Invalid path %s", source)
	}

	if source[0] == '%' {
		parts := strings.SplitN(source, string(os.PathSeparator), 2)
		sandbox, err := getSandboxById(s, parts[0])
		if err != nil {
			return source, err
		}
		if len(parts) == 2 {
			return filepath.Join(sandbox.Path, parts[1]), nil
		}
		return sandbox.Path, nil
	}

	if !filepath.IsAbs(source) {
		return source, fmt.Errorf("Relative path %s", source)
	}

	if restricted {
		_, err := getSandboxByPath(s, source)
		if err != nil {
			return source, err
		}
	}

	return source, nil
}
