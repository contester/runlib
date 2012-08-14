package service

import (
	"code.google.com/p/goprotobuf/proto"
	"fmt"
	"os"
	"path/filepath"
	"runlib/contester_proto"
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

func statFile(name string) *contester_proto.StatResponse_StatFile {
	f := &contester_proto.StatResponse_StatFile{}
	f.Name = &name
	info, err := os.Stat(name)
	if err == nil {
		f.Exists = proto.Bool(true)
		if info.IsDir() {
			f.IsDirectory = proto.Bool(true)
		}
	}
	return f
}

func (s *Contester) Stat(request *contester_proto.StatRequest, response *contester_proto.StatResponse) error {
	response.Stats = make([]*contester_proto.StatResponse_StatFile, len(request.Name))
	for index, name := range request.Name {
		response.Stats[index] = statFile(name)
	}
	return nil
}

func (s *Contester) Glob(request *contester_proto.GlobRequest, response *contester_proto.GlobResponse) (err error) {
	response.Results = make([]*contester_proto.GlobResponse_SingleGlob, len(request.Expression))
	for index, expr := range request.Expression {
		f := &contester_proto.GlobResponse_SingleGlob{Expression: proto.String(expr)}
		var e1 error
		f.Results, e1 = filepath.Glob(expr)
		if e1 != nil && err == nil {
			err = e1
		}
		response.Results[index] = f
	}

	return
}
