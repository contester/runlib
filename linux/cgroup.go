package linux

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"

	log "github.com/Sirupsen/logrus"
	"github.com/contester/runlib/tools"
)

type Cgroups struct {
	cpuacct, memory string
}

func parseProcCgroups(r io.Reader) map[string]string {
	result := make(map[string]string)
	s := bufio.NewScanner(r)
	for s.Scan() {
		if line := s.Text(); line != "" {
			splits := strings.SplitN(line, ":", 3)
			items := strings.Split(splits[1], ",")

			for _, v := range items {
				result[v] = splits[2]
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	return nil
}

func parseProcMounts(r io.Reader, cgroups map[string]string) map[string]string {
	result := make(map[string]string)

	s := bufio.NewScanner(r)
	for s.Scan() {
		if line := s.Text(); line != "" {
			splits := strings.SplitN(line, " ", 6)
			if splits[2] != "cgroup" {
				continue
			}
			opts := strings.Split(splits[3], ",")
			for _, opt := range opts {
				if _, ok := cgroups[opt]; ok {
					result[opt] = splits[1]
				}
			}
		}
	}
	if len(result) > 0 {
		return result
	}
	return nil
}

func openAndParse(filename string, parser func(io.Reader) map[string]string) (map[string]string, error) {
	if f, err := os.Open(filename); err == nil {
		defer f.Close()
		return parser(f), nil
	} else {
		return nil, tools.NewError(err, "openAndParse", "os.Open")
	}
}

func combineCgPmap(procmap, cgmap map[string]string, name string) string {
	if procmap[name] != "" && cgmap[name] != "" {
		return procmap[name] + cgmap[name]
	}
	return ""
}

func NewCgroups() (*Cgroups, error) {
	cgmap, err := openAndParse("/proc/self/cgroup", parseProcCgroups)
	if err != nil {
		return nil, err
	}

	procmap, err := openAndParse("/proc/mounts", func(r io.Reader) map[string]string {
		return parseProcMounts(r, cgmap)
	})

	if err != nil {
		return nil, err
	}

	var result Cgroups
	result.memory = combineCgPmap(procmap, cgmap, "memory")
	result.cpuacct = combineCgPmap(procmap, cgmap, "cpuacct")

	if result.memory != "" || result.cpuacct != "" {
		return &result, nil
	}

	return nil, fmt.Errorf("Cannot attach to cpuacct and memory cgroups")
}

// check if cgroup exists.

func cgAttach(name string, pid int) error {
	f, err := os.Create(name + "/tasks")
	if err != nil {
		log.Error("Can't attach to cgroup %s, pid %d: %s", name, pid, err)
		return err
	}
	defer f.Close()
	_, err = f.WriteString(strconv.Itoa(pid) + "\n")
	if err != nil {
		log.Error("Can't attach to cgroup %s, pid %d: %s", name, pid, err)
		return err
	}
	return nil
}

func cgSetup(name string, pid int) error {
	_, err := os.Stat(name)
	if tools.IsStatErrorFileNotFound(err) {
		err = os.MkdirAll(name, os.ModeDir|0755)
		if err != nil {
			log.Error(err)
			return err
		}
	}
	return cgAttach(name, pid)
}

func (c *Cgroups) Setup(name string, pid int) error {
	errCpu := cgSetup(c.cpuacct+"/"+name, pid)
	errMemory := cgSetup(c.memory+"/"+name, pid)

	if errCpu != nil && errMemory != nil {
		return errCpu
	}
	return nil
}

func (c *Cgroups) Remove(name string) error {
	errCpu := syscall.Rmdir(c.cpuacct + "/" + name)
	errMemory := syscall.Rmdir(c.memory + "/" + name)
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
	return cgRead1u64(c.memory+"/"+name, "memory.max_usage_in_bytes")
}

func (c *Cgroups) GetCpu(name string) uint64 {
	return cgRead1u64(c.cpuacct+"/"+name, "cpuacct.usage")
}
