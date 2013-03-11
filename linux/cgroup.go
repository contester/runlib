package linux

import (
	"bufio"
	"path/filepath"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// Parse /proc/self/cgroup

const (
	CG = "/sys/fs/cgroup"
	PROC_SELF_CGROUP = "/proc/self/cgroup"
)

type Cgroups struct {
	CpuAcct string
	Memory string
}

func parseProcCgroups(r io.Reader) (result map[string]string) {
	result = make(map[string]string)
	b := bufio.NewReader(r)
	for {
		line, err := b.ReadString("\n")
		if line != "" {
			splits := strings.SplitN(line, ":", 3)
			items := strings.Split(splits[1], ",")

			for _, v := range items {
				result[v] = splits[2]
			}
		}

		if err != nil {
			break
		}
	}
	return
}

func cgmapget(m map[string]string, v string) string {
	if p, ok = m[v]; ok {
		return CG + "/" + v + p
	}
	return ""
}

func NewCgroups() (*Cgroups) {
	result := &Cgroups{}

	ifile, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return
	}

	cgmap := parseProcCgroups(ifile)
	ifile.Close()

	if cgmap == nil {
		return
	}

	result.Memory = cgmapget(cgmap, "memory")
	result.CpuAcct = cgmapget(cgmap, "cpuacct")
}

// check if cgroup exists.
//

func cgAttach(name string, pid int) error {
	f, err := os.Create(name + "/tasks")
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strconv.Itoa(pid) + "\n")
	if err != nil {
		return err
	}
	return nil
}

func cgSetup(name string, pid int) error {
	_, err := os.Stat(name)
	if err != nil {
		if errno, ok := err.(syscall.Errno); ok && errno == syscall.ENOENT {
			err = os.MkdirAll(name, os.ModeDir)
			if err != nil {
				return err
			}
			return cgAttach(name, pid)
		}
		return err
	}
	return cgAttach(name, pid)
}

func (c *Cgroups) Setup(name string, pid int) error {
	errCpu := cgSetup(c.CpuAcct + "/contester/" + name, pid)
	errMemory := cgSetup(c.Memory + "/contester/" + name, pid)

	if errCpu != nil && errMemory != nil {
		return errCpu
	}
	return nil
}

func (c *Cgroups) Remove(name string) error {
	errCpu := syscall.Rmdir(c.CpuAcct + "/contester/" + name)
	errMemory := syscall.Rmdir(c.Memory + "/contester/" + name)
	if errCpu != nil && errMemory != nil {
		return errCpu
	}
	return nil
}

func cgRead1u64(name, metric string) uint64 {
	f, err := os.Open(name + "/" + metric)
	if err != nil {
		return 0
	}
	b := bufio.NewReader(f)
	s, _, err := b.ReadLine()
	if err != nil {
		return 0
	}
	r, err := strconv.ParseUint(string(s), 10, 64)
	if err != nil {
		return 0
	}
	return r
}

func (c *Cgroups) GetMemory(name string) uint64 {
	return cgRead1u64(c.Memory + "/contester/" + name, "memory.max_usage_in_bytes")
}

func (c *Cgroups) GetCpu(name string) uint64 {
	return cgRead1u64(c.CpuAcct + "/contester/" + name, "cpuacct.usage")
}
