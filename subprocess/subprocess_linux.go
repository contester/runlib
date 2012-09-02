package subprocess

import (
	"os/user"
	"strconv"
	"runlib/linux"
	"fmt"
)

type LoginInfo struct {
	Uid int
}

type PlatformOptions struct {}

type PlatformData struct {
	Pid int
	params *linux.CloneParams
}

func NewLoginInfo(username, password string) (*LoginInfo, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, err
	}
	return &LoginInfo{
		Uid: uid,
	}, nil
}

func (d *subprocessData) wAllRedirects(s *Subprocess, result *linux.StdHandles) error {
	var err error

	if result.StdIn, err = d.SetupInput(s.StdIn); err != nil {
		return err
	}
	if result.StdOut, err = d.SetupOutput(s.StdOut, &d.stdOut); err != nil {
		return err
	}
	if result.StdErr, err = d.SetupOutput(s.StdErr, &d.stdErr); err != nil {
		return err
	}
	return nil
}


func (sub *Subprocess) CreateFrozen() (*subprocessData, error) {
	if sub.Cmd.ApplicationName == nil {
		return nil, fmt.Errorf("Application name must be present")
	}
	d := &subprocessData{}
	var stdh linux.StdHandles
	err := d.wAllRedirects(sub, &stdh)
	if err != nil {
		stdh.Close()
		return nil, err
	}
	d.platformData.params, err = linux.CreateCloneParams(*sub.Cmd.ApplicationName, sub.Cmd.Parameters, *sub.Environment, *sub.CurrentDirectory, sub.Login.Uid, stdh)
	return d, nil
}
