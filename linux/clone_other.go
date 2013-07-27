package linux

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"
)

var childStages = map[int]string{
	1: "chdir",
	2: "setuid",
	3: "ptrace",
	4: "exec",
}

type StdHandles struct {
	StdIn, StdOut, StdErr *os.File
}

type CommStatus struct {
	What int
	Err  error
}

func (s *StdHandles) Close() {
	if s.StdIn != nil {
		s.StdIn.Close()
	}
	if s.StdOut != nil {
		s.StdOut.Close()
	}
	if s.StdErr != nil {
		s.StdErr.Close()
	}
}

func commReader(r *os.File, sig chan CommStatus) {
	defer r.Close()
	for {
		var buf [2]int32
		err := binary.Read(r, binary.LittleEndian, buf[:])
		if err != nil {
			if err == io.EOF {
				close(sig)
				return
			}
			var v CommStatus
			v.What = 0
			v.Err = err
			sig <- v
			close(sig)
			return
		}
		var v CommStatus
		v.What = int(buf[0])
		v.Err = syscall.Errno(buf[1])
		sig <- v
	}
}

func childError(c CommStatus) error {
	w, ok := childStages[c.What]
	if !ok {
		w = strconv.Itoa(c.What)
	}
	return os.NewSyscallError(w, c.Err)
}

func (c *CloneParams) CloneFrozen() (int, error) {
	pid := callClone(c)
	// TODO: clone errors?
	c.CommWriter.Close()
	c.stdhandles.Close()
	c.comm = make(chan CommStatus)
	go commReader(c.CommReader, c.comm)

	var status syscall.WaitStatus
	for {
		wpid, err := syscall.Wait4(pid, &status, 0, nil) // TODO: rusage
		if err != nil {
			return -1, os.NewSyscallError("Wait4", err)
		}
		if wpid == pid {
			break
		}
	}
	if status.Stopped() && status.StopSignal() == syscall.SIGTRAP {
		return pid, nil
	}
	if status.Exited() {
		co, ok := <-c.comm
		if ok {
			return -1, childError(co)
		}
		return -1, fmt.Errorf("DAFUQ")
	}
	err := syscall.Kill(pid, syscall.SIGKILL)
	if err != nil {
		return -1, os.NewSyscallError("Kill", err)
	}
	return -1, fmt.Errorf("traps, signals, dafuq is this")
}

func (c *CloneParams) Unfreeze(pid int) error {
	err := syscall.PtraceDetach(pid)
	co, ok := <-c.comm
	if !ok {
		return os.NewSyscallError("PtraceDetach", err)
	}
	return childError(co)
}
