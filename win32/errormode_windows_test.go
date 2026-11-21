package win32

import "testing"

func TestWin32ToNtHardErrorMode(t *testing.T) {
	cases := []struct {
		name string
		in   uint32
		want uint32
	}{
		{"zero", 0, 0x0001},
		{"fail critical errors only", SEM_FAILCRITICALERRORS, 0x0000},
		{"no gpf error box only", SEM_NOGPFAULTERRORBOX, 0x0003},
		{"suppress all dialogs mask", SuppressErrorDialogsMask, 0x8002},
		{"with alignment fault flag", SuppressErrorDialogsMask | SEM_NOALIGNMENTFAULTEXCEPT, 0x8006},
	}

	for _, c := range cases {
		if got := win32ToNtHardErrorMode(c.in); got != c.want {
			t.Errorf("%s: win32ToNtHardErrorMode(0x%04x) = 0x%04x, want 0x%04x", c.name, c.in, got, c.want)
		}
		if roundtrip := win32ToNtHardErrorMode(win32ToNtHardErrorMode(c.in)); roundtrip != c.in {
			t.Errorf("%s: involution failed: got 0x%04x, want 0x%04x", c.name, roundtrip, c.in)
		}
	}
}
