package service

import (
	"code.google.com/p/goconf/conf"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"code.google.com/p/goprotobuf/proto"
)

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
	return os.MkdirAll(s.Path, os.ModeDir)
}
