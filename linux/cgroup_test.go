package linux

import (
	"testing"
	"bytes"
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
	checkMap(t, parsed, 9, map[string]string {
			"cpuacct": "/user/stingray/5",
			"memory": "/user/stingray/5",
			"devices": "/",
		})
}
