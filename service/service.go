package service

import (
	"code.google.com/p/goprotobuf/proto"
	"labix.org/v2/mgo"
	"os"
	"runlib/contester_proto"
	"strings"
	"runlib/platform"
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
	result := make([]*contester_proto.LocalEnvironment_Variable,len(list))
	for i, v := range list {
		s := strings.SplitN(v, "=", 2)
		result[i] = &contester_proto.LocalEnvironment_Variable{
			Name:  proto.String(s[0]),
			Value: proto.String(s[1])}
	}
	return result
}

func NewContester(configFile string, gData *platform.GlobalData) *Contester {
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
		GData: gData,
	}

	return result
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
	response.Sandboxes = make([]*contester_proto.SandboxLocations,len(s.Sandboxes))
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
