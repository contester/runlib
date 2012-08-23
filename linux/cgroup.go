package linux

// +build linux

import (
	"os"
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

func CgCreate(name string) {
	CgCreate1(CG_CPU, name)
	CgCreate1(CG_MEMORY, name)
}
