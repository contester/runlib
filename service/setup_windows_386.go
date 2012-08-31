package service

import (
	"code.google.com/p/goconf/conf"
	"os"
	"os/exec"
	"path/filepath"
	"runlib/subprocess"
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

		e := checkSandbox(result[index].Compile.Path)
		if e != nil {
			return nil, e
		}
		e = checkSandbox(result[index].Run.Path)
		if e != nil {
			return nil, e
		}
		e = setAcl(result[index].Run.Path, "tester"+strconv.Itoa(index))
		if e != nil {
			return nil, e
		}
		result[index].Run.Login, e = subprocess.NewLoginInfo("tester"+strconv.Itoa(index), password)
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

func setAcl(path, username string) error {
	cmd := exec.Command("subinacl.exe", "/file", path, "/grant="+username+"=RWC")
	cmd.Run()
	return nil
}
