package service

import (
	"os"
	"path/filepath"
	"strings"
	//  "fmt"
	"code.google.com/p/goconf/conf"
	"code.google.com/p/goprotobuf/proto"
	"runlib/contester_proto"
)

type Sandbox struct {
	Path               string
	Username, Password *string
}

type SandboxPair struct {
	Compile, Run Sandbox
}

type Contester struct {
	InvokerId string
	Sandboxes []SandboxPair
}

type ContesterConfig struct {
	BasePath            string
	RestrictedPasswords []string
}

func getHostname() string {
	result, err := os.Hostname()
	if err != nil {
		return "undefined"
	}
	return result
}

func readConfigFile(configFile string) (*ContesterConfig, error) {
	c, err := conf.ReadConfigFile(configFile)
	if err != nil {
		return nil, err
	}
	result := &ContesterConfig{}
	result.BasePath, err = c.GetString("default", "path")
	if err != nil {
		return nil, err
	}

	passwords, err := c.GetString("default", "passwords")
	if err != nil {
		return nil, err
	}

	result.RestrictedPasswords = strings.Split(passwords, " ")

	return result, nil
}

func configureSandboxes(conf *ContesterConfig) ([]SandboxPair, error) {
	result := make([]SandboxPair, len(conf.RestrictedPasswords))
	for index, password := range conf.RestrictedPasswords {
		localBase := filepath.Join(conf.BasePath, string(index))
		result[index].Compile.Path = filepath.Join(localBase, "C")
		result[index].Run.Path = filepath.Join(localBase, "R")
		result[index].Run.Username = proto.String("tester" + string(index))
		result[index].Run.Password = proto.String(password)
	}
	return result, nil
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
		InvokerId: getHostname(),
		Sandboxes: sandboxes,
	}

	return result
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	response.InvokerId = &s.InvokerId

	return nil
}

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.StatResponse) error {
	name := request.GetName()

	response.Name = &name
	info, err := os.Stat(name)
	if err == nil {
		response.Exists = proto.Bool(true)
		if info.IsDir() {
			response.IsDirectory = proto.Bool(true)
		}
	}

	return nil
}
