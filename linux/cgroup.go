package linux

import (
	"os"
	"syscall"
	"strconv"
	"bufio"
)

const (
	CG_MEMORY = "/sys/fs/cgroup/memory"
	CG_CPU    = "/sys/fs/cgroup/cpuacct"
)

func CgFormat(prefix, name string) string {
	return prefix + "/contester/" + name
}

func CgCreate1(prefix, name string) {
	os.MkdirAll(CgFormat(prefix, name), os.ModeDir)
}

func CgRemove1(prefix, name string) error {
	return syscall.Rmdir(CgFormat(prefix, name))
}

func CgCreate(name string) {
	CgCreate1(CG_CPU, name)
	CgCreate1(CG_MEMORY, name)
}

func CgRemove(name string) {
	CgRemove1(CG_CPU, name)
	CgRemove1(CG_MEMORY, name)
}

func CgAttach1(prefix, name string, pid int) error {
	f, err := os.Create(CgFormat(prefix, name) + "/tasks")
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

func CgAttach(name string, pid int) error {
	CgAttach1(CG_CPU, name, pid)
	CgAttach1(CG_MEMORY, name, pid)
	return nil
}

func CgRead1u64(prefix, name, metric string) uint64 {
	f, err := os.Open(CgFormat(prefix, name) + "/" + metric)
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

func CgGetMemory(name string) uint64 {
	return CgRead1u64(CG_MEMORY, name, "memory.max_usage_in_bytes")
}

func CgGetCpu(name string) uint64 {
	return CgRead1u64(CG_CPU, name, "cpuacct.usage")
}
