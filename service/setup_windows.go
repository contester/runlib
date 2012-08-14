package service

import (
	"code.google.com/p/goconf/conf"
	"code.google.com/p/goprotobuf/proto"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type ContesterConfig struct {
	BasePath            string
	RestrictedPasswords []string
	Server              string
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

	if result.Server, err = c.GetString("default", "server"); err != nil {
		return nil, err
	}

	return result, nil
}

func configureSandboxes(conf *ContesterConfig) ([]SandboxPair, error) {
	result := make([]SandboxPair, len(conf.RestrictedPasswords))
	for index, password := range conf.RestrictedPasswords {
		localBase := filepath.Join(conf.BasePath, strconv.Itoa(index))
		result[index].Compile.Path = filepath.Join(localBase, "C")
		result[index].Run.Path = filepath.Join(localBase, "R")
		result[index].Run.Username = proto.String("tester" + strconv.Itoa(index))
		result[index].Run.Password = proto.String(password)

		e := checkSandbox(&result[index].Compile)
		if e != nil {
			return nil, e
		}
		e = checkSandbox(&result[index].Run)
		if e != nil {
			return nil, e
		}
	}
	return result, nil
}

func checkSandbox(s *Sandbox) error {
	err := os.MkdirAll(s.Path, os.ModeDir)
	if err != nil {
		return err
	}
	if s.Username != nil {
		cmd := exec.Command("subinacl.exe", "/file", s.Path, "/grant="+*s.Username+"=RWC")
		cmd.Run()
	}
	return nil
}
