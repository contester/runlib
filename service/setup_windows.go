package service

import (
	"errors"
	"hash/fnv"
	"math/rand"
	"os/exec"
	"runtime"
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

func pwgen1(src *rand.Rand, n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, n)
	for i := range result {
		result[i] = alphabet[src.Intn(len(alphabet))]
	}
	return string(result)
}

func getPasswords(c *contesterConfig) []string {
	if c.Passwords != "" {
		strings.Split(c.Passwords, " ")
	}
	cores := c.SandboxCount
	if cores == 0 {
		// Getting real CPU count is too hard, let's just assume we have HT on.
		cores = (runtime.NumCPU() / 2) - 1
	}
	if cores < 1 {
		cores = 1
	}
	result := make([]string, cores)

	mh := fnv.New64()
	mh.Write([]byte(getHostname()))
	src := rand.New(rand.NewSource(int64(mh.Sum64())))

	for i := range result {
		result[i] = pwgen1(src, 8)
	}
	return result
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
