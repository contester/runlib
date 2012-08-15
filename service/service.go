package service

import (
	"code.google.com/p/goprotobuf/proto"
	"io/ioutil"
	"os"
	"path"
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

func (s *Contester) Put(request *contester_proto.PutRequest, response *contester_proto.PutResponse) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetSandbox())
	if err != nil {
		return err
	}

	for _, module := range request.Module {
		destPath := filepath.Join(sandbox.Path, module.GetName())
		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}
		data, err := module.Data.Bytes()
		if err != nil {
			return err
		}
		_, err = destFile.Write(data)
		if err != nil {
			return err
		}
		destFile.Close()
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

func (s *Contester) Get(request *contester_proto.GetRequest, response *contester_proto.GetResponse) error {
	sandbox, err := getSandboxById(s.Sandboxes, request.GetPrefix())

	if err != nil {
		return err
	}

	response.Module = make([]*contester_proto.Module, 0, len(request.Filename))

	basepath := sandbox.Path
	for _, name := range request.Filename {
		fullpath := filepath.Join(basepath, name)
		data, err := ioutil.ReadFile(fullpath)
		if err != nil {
			continue
		}
		blob, _ := contester_proto.NewBlob(data)

		module := &contester_proto.Module{
			Data: blob,
			Name: proto.String(name),
			Type: proto.String(path.Ext(name)[1:]),
		}
		response.Module = response.Module[:len(response.Module)+1]
		response.Module[len(response.Module)-1] = module
	}
	return nil
}
