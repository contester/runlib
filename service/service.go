package service

import (
	"code.google.com/p/goconf/conf"
	"code.google.com/p/goprotobuf/proto"
	"labix.org/v2/mgo"
	"os"
	"path/filepath"
	"runlib/contester_proto"
	"runlib/platform"
	"runlib/subprocess"
	"strconv"
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

	GData *platform.GlobalData

	Msession  *mgo.Session
	Mlocation string
	Mdb       *mgo.Database
	Mfs       *mgo.GridFS
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

func configureSandboxes(config *conf.ConfigFile) ([]SandboxPair, error) {
	basePath, err := config.GetString("default", "path")
	if err != nil {
		return nil, err
	}
	passwords, err := getPasswords(config)
	if err != nil {
		return nil, err
	}

	result := make([]SandboxPair, len(passwords))
	for index, password := range passwords {
		localBase := filepath.Join(basePath, strconv.Itoa(index))
		result[index].Compile.Path = filepath.Join(localBase, "C")
		result[index].Run.Path = filepath.Join(localBase, "R")

		e := checkSandbox(result[index].Compile.Path)
		if e != nil {
			return nil, e
		}
		e = checkSandbox(result[index].Run.Path)
		if e != nil {
			return nil, e
		}

		restrictedUser := "tester" + strconv.Itoa(index)

		e = setAcl(result[index].Run.Path, restrictedUser)
		if e != nil {
			return nil, e
		}
		// HACK HACK: on linux, passwords are ignored.
		result[index].Run.Login, e = subprocess.NewLoginInfo(restrictedUser, password)
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}

func checkSandbox(path string) error {
	err := os.MkdirAll(path, os.ModeDir)
	if err != nil {
		return err
	}
	return nil
}

func NewContester(configFile string, gData *platform.GlobalData) (*Contester, error) {
	config, err := conf.ReadConfigFile(configFile)
	if err != nil {
		return nil, err
	}

	var result Contester

	result.InvokerId = getHostname()
	result.Env = getLocalEnvironment()
	result.ServerAddress, err = config.GetString("default", "server")
	if err != nil {
		return nil, err
	}
	result.Platform = PLATFORM_ID
	result.Disks = PLATFORM_DISKS
	result.ProgramFiles = PLATFORM_PFILES
	result.PathSeparator = string(os.PathSeparator)
	result.GData = gData

	result.Sandboxes, err = configureSandboxes(config)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	mLocation := *request.MongoHost
	if mLocation != s.Mlocation {
		var err error
		s.Msession, err = mgo.Dial(*request.MongoHost)
		if err != nil {
			return err
		}
	}
	s.Mdb = s.Msession.DB(*request.MongoDb)
	s.Mfs = s.Mdb.GridFS("fs")

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
