package subprocess

import "testing"

func TestUseLogonLaunch(t *testing.T) {
	cases := []struct {
		name                         string
		noJob, win8, useAsUser, want bool
	}{
		{"win8 solution, no asuser -> logon", false, true, false, true},
		{"win8 tester, no asuser -> logon", true, true, false, true},
		{"win8 solution, asuser -> asuser", false, true, true, false},
		// The regression this change fixes: a NoJob tester on Win8 with
		// UseAsUser must NOT fall back to the per-execute logon.
		{"win8 tester, asuser -> asuser", true, true, true, false},
		{"pre-win8 solution -> asuser (legacy)", false, false, false, false},
		{"pre-win8 tester -> logon", true, false, false, true},
		{"pre-win8 tester, asuser -> asuser", true, false, true, false},
	}
	for _, c := range cases {
		if got := useLogonLaunch(c.noJob, c.win8, c.useAsUser); got != c.want {
			t.Errorf("%s: useLogonLaunch(noJob=%v, win8=%v, useAsUser=%v) = %v, want %v",
				c.name, c.noJob, c.win8, c.useAsUser, got, c.want)
		}
	}
}
