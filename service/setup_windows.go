package service

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/contester/runlib/win32"
	"golang.org/x/sys/windows"

	log "github.com/sirupsen/logrus"
)

// procAdjustTokenPrivileges is called directly (rather than via
// windows.AdjustTokenPrivileges) so enablePrivilege can read the errno that the
// call captures atomically; the x/sys wrapper discards it on success.
var procAdjustTokenPrivileges = windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")

// Privileges CreateProcessAsUser requires in the caller. Administrators hold
// SeIncreaseQuotaPrivilege by default but NOT SeAssignPrimaryTokenPrivilege;
// the latter is granted via "Replace a process level token" in the local
// security policy. x/sys/windows does not export these names as constants.
const (
	seAssignPrimaryTokenPrivilege = "SeAssignPrimaryTokenPrivilege"
	seIncreaseQuotaPrivilege      = "SeIncreaseQuotaPrivilege"
)

// enablePrivilege enables a named privilege in the current process token.
// It returns ERROR_NOT_ALL_ASSIGNED when the account does not hold the
// privilege — AdjustTokenPrivileges reports that only via the last error, not
// its return value.
func enablePrivilege(name string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()

	np, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, np, &luid); err != nil {
		return err
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}

	// AdjustTokenPrivileges succeeds (r1 != 0) even when the token does not hold
	// the privilege; it only signals that via ERROR_NOT_ALL_ASSIGNED in the
	// last error. proc.Call captures that errno atomically with the call (e1),
	// which a separate windows.GetLastError() cannot do reliably.
	r1, _, e1 := procAdjustTokenPrivileges.Call(
		uintptr(token), 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	if r1 == 0 {
		return fmt.Errorf("AdjustTokenPrivileges(%s): %w", name, e1)
	}
	if errors.Is(e1, windows.ERROR_NOT_ALL_ASSIGNED) {
		return fmt.Errorf("%s not held by the account: %w", name, e1)
	}
	return nil
}

// grantEveryoneCurrentDesktop lets processes launched as the sandbox users
// (via CreateProcessAsUser) attach to the invoker's own, visible window
// station and desktop (winsta0\default), so their consoles stay visible. The
// grant is additive (see win32.*Keep) and so leaves the existing desktop
// security intact.
func grantEveryoneCurrentDesktop() error {
	everyone, err := syscall.StringToSid("S-1-1-0")
	if err != nil {
		return err
	}
	winsta, err := win32.GetProcessWindowStation()
	if err != nil {
		return err
	}
	if err := win32.AddAceToWindowStationKeep(winsta, everyone); err != nil {
		return err
	}
	desk, err := win32.GetThreadDesktop(windows.GetCurrentThreadId())
	if err != nil {
		return err
	}
	return win32.AddAceToDesktopKeep(desk, everyone)
}

// tryEnableAsUser enables the privileges CreateProcessAsUser needs and, when
// they are available, grants sandbox users access to the visible desktop. It
// returns true only when restricted runs can safely use CreateProcessAsUser
// with the cached logon token instead of a per-execute CreateProcessWithLogonW;
// on any failure it returns false and the invoker keeps the existing path.
func tryEnableAsUser() bool {
	if err := enablePrivilege(seAssignPrimaryTokenPrivilege); err != nil {
		log.Infof("CreateProcessAsUser disabled: %s not held (%v); using CreateProcessWithLogonW", seAssignPrimaryTokenPrivilege, err)
		return false
	}
	if err := enablePrivilege(seIncreaseQuotaPrivilege); err != nil {
		log.Infof("CreateProcessAsUser disabled: %s not held (%v); using CreateProcessWithLogonW", seIncreaseQuotaPrivilege, err)
		return false
	}
	if err := grantEveryoneCurrentDesktop(); err != nil {
		log.Errorf("CreateProcessAsUser disabled: desktop grant failed (%v); using CreateProcessWithLogonW", err)
		return false
	}
	log.Info("CreateProcessAsUser enabled for restricted runs: token privileges held, desktop access granted")
	return true
}

const PLATFORM_ID = "win32"

var PLATFORM_DISKS = []string{"C:\\"}
var PLATFORM_PFILES = []string{"C:\\Program Files", "C:\\Program Files (x86)"}

func setAcl(path, username string) error {
	// Grant the sandbox user Modify on its sandbox dir, inheritable so the
	// files the invoker drops in (solution binary, input.txt) and the ones the
	// solution writes (output.txt) carry the grant. icacls ships with every
	// Windows; subinacl.exe did not, and its failure was swallowed — leaving
	// restricted sandboxes with no ACE for the tester user.
	out, err := exec.Command("icacls", path, "/grant", username+":(OI)(CI)M").CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls grant %s on %q: %w: %s", username, path, err, out)
	}
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
