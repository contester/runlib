package service

import (
	"os/exec"
	"strings"

	"code.google.com/p/goconf/conf"
)

const PLATFORM_ID = "win32"

var PLATFORM_DISKS = []string{"C:\\"}
var PLATFORM_PFILES = []string{"C:\\Program Files", "C:\\Program Files (x86)"}

func getPasswords(c *conf.ConfigFile) ([]string, error) {
	passwords, err := c.GetString("default", "passwords")
	if err != nil {
		return nil, err
	}
	return strings.Split(passwords, " "), nil
}

func setAcl(path, username string) error {
	cmd := exec.Command("subinacl.exe", "/file", path, "/grant="+username+"=RWC")
	cmd.Run()
	return nil
}
