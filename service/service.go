package service

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/contester/runlib/contester_proto"
	"github.com/contester/runlib/platform"
	"github.com/contester/runlib/storage"
	"github.com/contester/runlib/subprocess"
	"gopkg.in/gcfg.v1"
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

	mu      sync.RWMutex
	Storage storage.Backend
}

func getHostname() string {
	if result, err := os.Hostname(); err == nil {
		return result
	}
	return "undefined"
}

func getLocalEnvironment() []*contester_proto.LocalEnvironment_Variable {
	list := os.Environ()
	result := make([]*contester_proto.LocalEnvironment_Variable, 0, len(list))
	for _, v := range list {
		s := strings.SplitN(v, "=", 2)
		result = append(result, &contester_proto.LocalEnvironment_Variable{
			Name:  s[0],
			Value: s[1]})
	}
	return result
}

func configureSandboxes(config *contesterConfig) ([]SandboxPair, error) {
	basePath := config.Default.Path
	passwords := getPasswords(config)
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

		if PLATFORM_ID == "linux" {
			e = setAcl(result[index].Compile.Path, "compiler")
			if e != nil {
				return nil, e
			}
			result[index].Compile.Login, e = subprocess.NewLoginInfo("compiler", "compiler")
			if e != nil {
				return nil, e
			}
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
	err := os.MkdirAll(path, os.ModeDir|0755)
	if err != nil {
		return err
	}
	return nil
}

type contesterConfig struct {
	Default struct {
		Server, Passwords, Path string
		SandboxCount            int
	}
}

func NewContester(configFile string, gData *platform.GlobalData) (*Contester, error) {
	var config contesterConfig
	if err := gcfg.ReadFileInto(&config, configFile); err != nil {
		return nil, err
	}

	log.Infof("Loaded contester config %+v", &config)

	var result Contester

	result.InvokerId = getHostname()
	result.Env = getLocalEnvironment()
	result.ServerAddress = config.Default.Server
	result.Platform = PLATFORM_ID
	result.Disks = PLATFORM_DISKS
	result.ProgramFiles = PLATFORM_PFILES
	result.PathSeparator = string(os.PathSeparator)
	result.GData = gData

	var err error
	result.Sandboxes, err = configureSandboxes(&config)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *Contester) Identify(request *contester_proto.IdentifyRequest, response *contester_proto.IdentifyResponse) error {
	backend, err := storage.NewBackend(request.GetMongoHost())
	if err != nil {
		return err
	}

	s.mu.Lock()
	if s.Storage != nil {
		s.Storage.Close()
	}
	s.Storage = backend
	s.mu.Unlock()

	response.InvokerId = s.InvokerId
	response.Environment = &contester_proto.LocalEnvironment{
		Variable: s.Env[:]}
	response.Sandboxes = make([]*contester_proto.SandboxLocations, 0, len(s.Sandboxes))
	for _, p := range s.Sandboxes {
		response.Sandboxes = append(response.Sandboxes, &contester_proto.SandboxLocations{
			Compile: p.Compile.Path,
			Run:     p.Run.Path})
	}
	response.Platform = s.Platform
	response.PathSeparator = s.PathSeparator
	response.Disks = s.Disks
	response.ProgramFiles = s.ProgramFiles

	return nil
}
