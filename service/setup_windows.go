package service

import (
	"errors"
	"os/exec"
	"strings"

	"github.com/contester/runlib/win32"
	"golang.org/x/sys/windows"
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

func ensureRestrictedUser(username, password string) error {
	err := win32.AddLocalUser(username, password)
	if err == nil {
		return nil
	}
	if !win32.IsAccountAlreadyExists(err) {
		return err
	}
	return nil
	// return win32.SetLocalUserPassword(username, password)
}

func isLogonFailure(err error) bool {
	return errors.Is(err, windows.ERROR_LOGON_FAILURE)
}

func maybeResetPassword(username, password string) error {
	return win32.SetLocalUserPassword(username, password)
}
