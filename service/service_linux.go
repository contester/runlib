package service

import (
	"code.google.com/p/goconf/conf"
	"os/exec"
	"strconv"
	"syscall"
)

const PLATFORM_ID = "linux"

var (
	PLATFORM_DISKS  = []string{"/"}
	PLATFORM_PFILES = []string{"/usr/bin", "/bin"}
)

func OnOsCreateError(err error) (bool, error) {
	return false, err
}

func getPasswords(c *conf.ConfigFile) ([]string, error) {
	count, err := c.GetInt("default", "sandboxCount")
	if err != nil {
		return nil, err
	}
	result := make([]string, count)
	for i := range result {
		result[i] = "password" + strconv.Itoa(i)
	}
	return result, nil
}

func setAcl(path, username string) error {
	cmd := exec.Command("chown", "-R", username, path)
	cmd.Run()
	cmd = exec.Command("chmod", "-R", "0700", path)
	cmd.Run()
	return nil
}

func IsFileNotFoundError(err error) bool {
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ENOENT {
			return true
		}
	}
	return false
}
