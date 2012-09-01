package subprocess

import (
	"os/user"
	"strconv"
)

type LoginInfo struct {
	Uid int
}

type PlatformOptions struct {}

type PlatformData struct {
	Pid int
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
