package service

import (
	"os/exec"
	"strings"
)

const PLATFORM_ID = "win32"

var PLATFORM_DISKS = []string{"C:\\"}
var PLATFORM_PFILES = []string{"C:\\Program Files", "C:\\Program Files (x86)"}

func setAcl(path, username string) error {
	cmd := exec.Command("subinacl.exe", "/file", path, "/grant="+username+"=RWC")
	cmd.Run()
	return nil
}

func getPasswords(c *contesterConfig) []string {
	return strings.Split(c.Default.Passwords, " ")
}
