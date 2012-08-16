package service

import (
	"code.google.com/p/goprotobuf/proto"
	"io/ioutil"
	"os"
	"path/filepath"
	"runlib/contester_proto"
	"strings"
)

type Contester struct {
	InvokerId     string
	Sandboxes     []SandboxPair
	Env           []*contester_proto.LocalEnvironment_Variable
	ServerAddress string

	Platform      string
	PathSeparator string
	Disks         []string
	ProgramFiles  []string
}

func getHostname() string {
	result, err := os.Hostname()
	if err != nil {
		return "undefined"
	}
	return result
}

func getLocalEnvironment() []*contester_proto.LocalEnvironment_Variable {
	list := os.Environ()
	result := make([]*contester_proto.LocalEnvironment_Variable, len(list))
	for i, v := range list {
		s := strings.SplitN(v, "=", 2)
		result[i] = &contester_proto.LocalEnvironment_Variable{
			Name:  proto.String(s[0]),
			Value: proto.String(s[1])}
	}
	return result
}

func NewContester(configFile string) *Contester {
	conf, err := readConfigFile(configFile)
	if err != nil {
		return nil
	}

	sandboxes, err := configureSandboxes(conf)
	if err != nil {
		return nil
	}

	result := &Contester{
		InvokerId:     getHostname(),
		Sandboxes:     sandboxes,
		Env:           getLocalEnvironment(),
		ServerAddress: conf.Server,
		Platform:      "win32",
		PathSeparator: string(os.PathSeparator),
		Disks:         []string{"C:\\"},
		ProgramFiles:  []string{"C:\\Program Files", "C:\\Program Files (x86)"},
	}

	return result
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	response.InvokerId = &s.InvokerId
	response.Environment = &contester_proto.LocalEnvironment{
		Variable: s.Env[:]}
	response.Sandboxes = make([]*contester_proto.SandboxLocations, len(s.Sandboxes))
	for i, p := range s.Sandboxes {
		response.Sandboxes[i] = &contester_proto.SandboxLocations{
			Compile: proto.String(p.Compile.Path),
			Run:     proto.String(p.Run.Path)}
	}
	response.Platform = &s.Platform
	response.PathSeparator = &s.PathSeparator
	response.Disks = s.Disks
	response.ProgramFiles = s.ProgramFiles

	return nil
}

func (s *Contester) Put(request *contester_proto.PutRequest, response *contester_proto.EmptyMessage) error {
	for _, item := range request.Files {
		resolved, err := resolvePath(s.Sandboxes, *item.Name, true)
		if err != nil {
			return err
		}
		dest, err := os.Create(resolved)
		if err != nil {
			return err
		}
		data, err := item.Data.Bytes()
		if err != nil {
			return err
		}
		_, err = dest.Write(data)
		if err != nil {
			return err
		}
		dest.Close()
	}
	return nil
}

func (s *Contester) Clear(request *contester_proto.ClearRequest, response *contester_proto.EmptyMessage) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	path := sandbox.Path
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return err
	}

	for _, info := range files {
		if info.Name() == "." || info.Name() == ".." {
			continue
		}
		fullpath := filepath.Join(path, info.Name())
		err = os.RemoveAll(fullpath)
		if err != nil {
			return err
		}
	}
	return nil
}

func getSingleFile(name string) (*contester_proto.FileBlob, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}
	blob, _ := contester_proto.NewBlob(data)

	return &contester_proto.FileBlob{
		Data: blob,
		Name: proto.String(name),
	}, nil
}

func (s *Contester) getSingleName(name string) (*contester_proto.FileContents, error) {
	stats, err := s.singleGlob(name)
	if err != nil || stats == nil {
		return nil, err
	}

	files := make([]*contester_proto.FileBlob, 0, len(stats.Results))

	for _, st := range stats.Results {
		if !*st.IsDirectory {
			f, err := getSingleFile(*st.Name)
			if err == nil && f != nil {
				files = append(files, f)
			}
		}
	}

	if len(files) > 0 {
		return &contester_proto.FileContents{Name: &name, Files: files}, nil
	}
	return nil, nil
}

func (s *Contester) Get(request *contester_proto.NameList, response *contester_proto.FileContentsList) error {
	response.Results = make([]*contester_proto.FileContents, 0, len(request.Name))

	for _, name := range request.Name {
		item, err := s.getSingleName(name)
		if err == nil && item != nil {
			response.Results = append(response.Results, item)
		}
	}
	return nil
}
