//go:build windows

package subprocess

import (
	"syscall"
	"testing"
)

func TestInheritHandleListDedupsJoinedOutput(t *testing.T) {
	// JoinStdOutErr makes StdErr alias StdOutput; the list must not repeat it.
	si := &syscall.StartupInfo{StdInput: 10, StdOutput: 20, StdErr: 20}
	got := inheritHandleList(si)
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("inheritHandleList = %v, want [10 20]", got)
	}
}

func TestInheritHandleListDropsInvalidAndZero(t *testing.T) {
	si := &syscall.StartupInfo{StdInput: syscall.InvalidHandle, StdOutput: 5, StdErr: 0}
	got := inheritHandleList(si)
	if len(got) != 1 || got[0] != 5 {
		t.Fatalf("inheritHandleList = %v, want [5]", got)
	}
}
