package service

import (
	"code.google.com/p/goprotobuf/proto"
	"os"
	"path/filepath"
	"runlib/contester_proto"
)

func statFile(name string) (*contester_proto.FileStat, error) {
	result := &contester_proto.FileStat{}
	result.Name = &name
	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		result.IsDirectory = proto.Bool(true)
	} else {
		result.Size = proto.Uint64(uint64(info.Size()))
	}
	return result, nil
}

func statFiles(names []string) ([]*contester_proto.FileStat, error) {
	result := make([]*contester_proto.FileStat, 0, len(names))
	for _, name := range names {
		stat, _ := statFile(name)
		if stat != nil {
			result = append(result, stat)
		}
	}
	if len(result) == 0 {
		result = nil
	}
	return result, nil
}

func statFilesAs(givenName string, names []string) (*contester_proto.FileStats, error) {
	stats, err := statFiles(names)
	if stats == nil || err != nil {
		return nil, err
	}

	return &contester_proto.FileStats{
		Name:    &givenName,
		Results: stats}, nil
}

func (s *Contester) Stat(request *contester_proto.NameList, response *contester_proto.FileStatsList) error {
	response.Results = make([]*contester_proto.FileStats, 0, len(request.Name))
	for _, name := range request.Name {
		resolved, err := resolvePath(s.Sandboxes, name, false)
		if err != nil {
			continue
		}
		stats, _ := statFilesAs(name, []string{resolved})
		if stats != nil {
			response.Results = append(response.Results, stats)
		}
	}
	return nil
}

func (s *Contester) singleGlob(expression string) (*contester_proto.FileStats, error) {
	resolved, err := resolvePath(s.Sandboxes, expression, false)
	if err != nil {
		return nil, err
	}
	names, err := filepath.Glob(resolved)
	if err != nil {
		return nil, err
	}
	result, err := statFilesAs(expression, names)
	return result, err
}

func (s *Contester) Glob(request *contester_proto.NameList, response *contester_proto.FileStatsList) error {
	response.Results = make([]*contester_proto.FileStats, 0, len(request.Name))
	for _, expr := range request.Name {
		value, _ := s.singleGlob(expr)
		if value != nil {
			response.Results = append(response.Results, value)
		}
	}

	return nil
}
