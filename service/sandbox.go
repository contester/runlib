package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/contester/runlib/subprocess"
	"github.com/juju/errors"
)

type Sandbox struct {
	Path  string
	Mutex sync.RWMutex
	Login *subprocess.LoginInfo
}

type SandboxPair struct {
	Compile, Run *Sandbox
}

func getSandboxById(s []SandboxPair, id string) (*Sandbox, error) {
	if len(id) < 4 || id[0] != '%' {
		return nil, errors.BadRequestf("Malformed sandbox ID %s", id)
	}
	parts := strings.Split(id[1:], ".")
	if len(parts) != 2 {
		return nil, errors.BadRequestf("Malformed sandbox ID %s", id)
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, errors.BadRequestf("Can't parse non-int sandbox index %s", parts[0])
	}

	if index < 0 || index >= len(s) {
		return nil, errors.BadRequestf("Sandbox index %d is out of range (max=%d)", index, len(s))
	}

	switch strings.ToUpper(parts[1]) {
	case "C":
		return s[index].Compile, nil
	case "R":
		return s[index].Run, nil
	}
	return nil, errors.BadRequestf("Sandbox variant %s is unknown", parts[1])
}

func getSandboxByPath(s []SandboxPair, id string) (*Sandbox, error) {
	cleanid := filepath.Clean(id)
	for _, v := range s {
		switch {
		case strings.HasPrefix(cleanid, v.Compile.Path):
			return v.Compile, nil
		case strings.HasPrefix(cleanid, v.Run.Path):
			return v.Run, nil
		}
	}
	return nil, errors.BadRequestf("No sandbox corresponds to path %s", cleanid)
}

func resolvePath(s []SandboxPair, source string, restricted bool) (string, *Sandbox, error) {
	if len(source) < 1 {
		return source, nil, errors.BadRequestf("Invalid path %s", source)
	}

	if source[0] == '%' {
		parts := strings.SplitN(source, string(os.PathSeparator), 2)
		sandbox, err := getSandboxById(s, parts[0])
		if err != nil {
			return source, nil, err
		}
		if len(parts) == 2 {
			return filepath.Join(sandbox.Path, parts[1]), sandbox, nil
		}
		return sandbox.Path, sandbox, nil
	}

	if !filepath.IsAbs(source) {
		return source, nil, errors.BadRequestf("Relative path %s", source)
	}

	if restricted {
		sandbox, err := getSandboxByPath(s, source)
		if err != nil {
			return source, nil, err
		}
		return source, sandbox, err
	}

	return source, nil, nil
}
