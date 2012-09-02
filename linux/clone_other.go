package linux

import (
	"os"
	"syscall"
	"fmt"
)

type StdHandles struct {
	StdIn, StdOut, StdErr *os.File
}

func (s *StdHandles) Close() {
	s.StdIn.Close()
	s.StdOut.Close()
	s.StdErr.Close()
}

//TODO: commreader goroutine

func (c *CloneParams) Clone() (int, error) {
	pid := callClone(c)
	// TODO: clone errors?
	c.CommWriter.Close()
	c.stdhandles.Close()

	var status syscall.WaitStatus
	for {
		wpid, err := syscall.Wait4(pid, &status, 0, nil) // TODO: rusage
		if err != nil {
			return -1, err
		}
		if wpid == pid {
			break
		}
	}
	if status.Stopped() && status.StopSignal() == syscall.SIGTRAP {
		// cgroup attach
		err := syscall.PtraceDetach(pid)
		if err != nil {
			// wtf to do here
		}
		return pid, nil
	}
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		return -1, err
	}
	return -1, fmt.Errorf("traps, signals, dafuq is this")
}
