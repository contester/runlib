package linux

import (
	"bytes"
	"testing"
)

const PROC_SELF_CGROUP = `9:perf_event:/
8:blkio:/user/stingray/5
7:net_cls:/user/stingray/5
6:freezer:/
5:devices:/
4:memory:/user/stingray/5
3:cpuacct,cpu:/user/stingray/5
2:cpuset:/
1:name=systemd:/user/stingray/5
`

const PROC_MOUNTS = `rootfs / rootfs rw 0 0
proc /proc proc rw,relatime 0 0
sysfs /sys sysfs rw,seclabel,relatime 0 0
devtmpfs /dev devtmpfs rw,seclabel,nosuid,size=5105524k,nr_inodes=1276381,mode=755 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
selinuxfs /sys/fs/selinux selinuxfs rw,relatime 0 0
tmpfs /dev/shm tmpfs rw,seclabel,relatime 0 0
devpts /dev/pts devpts rw,seclabel,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,seclabel,nosuid,nodev,mode=755 0 0
tmpfs /sys/fs/cgroup tmpfs rw,seclabel,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,release_agent=/usr/lib/systemd/systemd-cgroups-agent,name=systemd 0 0
cgroup /sys/fs/cgroup/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpuacct,cpu 0 0
cgroup /sys/fs/cgroup/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup/net_cls cgroup rw,nosuid,nodev,noexec,relatime,net_cls 0 0
cgroup /sys/fs/cgroup/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup /sys/fs/cgroup/perf_event cgroup rw,nosuid,nodev,noexec,relatime,perf_event 0 0
/dev/mapper/vg_vugluskr-root / ext4 rw,seclabel,relatime,data=ordered 0 0
systemd-1 /proc/sys/fs/binfmt_misc autofs rw,relatime,fd=33,pgrp=1,timeout=300,minproto=5,maxproto=5,direct 0 0
mqueue /dev/mqueue mqueue rw,seclabel,relatime 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,seclabel,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
tmpfs /tmp tmpfs rw,seclabel 0 0
sunrpc /var/lib/nfs/rpc_pipefs rpc_pipefs rw,relatime 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0
sunrpc /proc/fs/nfsd nfsd rw,relatime 0 0
/dev/mapper/vg_vugluskr-monvuvuzela /data/monvuvuzela ext4 rw,seclabel,noatime,nodiratime 0 0
/dev/md0 /boot ext4 rw,seclabel,relatime,data=ordered 0 0
/dev/sdd3 /mnt/btrfs btrfs rw,seclabel,noatime,nodiratime,degraded,space_cache,autodefrag,inode_cache 0 0
fusectl /sys/fs/fuse/connections fusectl rw,relatime 0 0
`

func checkItem(t *testing.T, x map[string]string, key, value string) {
	xk := x[key]
	if xk != value {
		t.Errorf("x[%s] (%s) != %s", key, xk, value)
	}
}

func checkMap(t *testing.T, x map[string]string, length int, items map[string]string) {
	if len(x) != length {
		t.Errorf("Length mismatch: %d must be %d", len(x), length)
	}

	for k, v := range items {
		checkItem(t, x, k, v)
	}
}

func TestPcgParse(t *testing.T) {
	parsed := parseProcCgroups(bytes.NewBufferString(PROC_SELF_CGROUP))
	checkMap(t, parsed, 10, map[string]string{
		"cpuacct": "/user/stingray/5",
		"memory":  "/user/stingray/5",
		"devices": "/",
	})
}

func TestPmParse(t *testing.T) {
	pcg := parseProcCgroups(bytes.NewBufferString(PROC_SELF_CGROUP))
	mp := parseProcMounts(bytes.NewBufferString(PROC_MOUNTS), pcg)
	checkMap(t, mp, 10, map[string]string{
		"cpuacct": "/sys/fs/cgroup/cpu,cpuacct",
		"memory":  "/sys/fs/cgroup/memory",
		"devices": "/sys/fs/cgroup/devices",
	})
}
