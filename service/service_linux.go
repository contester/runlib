package service

import (
	"os/exec"
	"strconv"
)

const PLATFORM_ID = "linux"

var (
	PLATFORM_DISKS  = []string{"/"}
	PLATFORM_PFILES = []string{"/usr/bin", "/bin"}
)

func OnOsCreateError(err error) (bool, error) {
	return false, err
}

func getPasswords(c *contesterConfig) []string {
	result := make([]string, c.Default.SandboxCount)
	for i := range result {
		result[i] = "password" + strconv.Itoa(i)
	}
	return result
}

func setAcl(path, username string) error {
	cmd := exec.Command("chown", "-R", username, path)
	cmd.Run()
	cmd = exec.Command("chmod", "-R", "0700", path)
	cmd.Run()
	return nil
}
