package service

import (
	"reflect"
	"strings"
	"testing"
)

func TestGeneratePasswordsMatchesSpec(t *testing.T) {
	cases := []struct {
		hostname string
		want     []string
	}{
		{"", []string{"uO9yvNZ95wnG", "pU8LrZKYtWDm", "jV1OQV32srSf", "dQ4fmbMlAF76"}},
		{"undefined", []string{"rW2YAejXBCwY", "dX1w0aTuHnZc", "iI8gs20ytVfY", "tB6mGJFg3w75"}},
		{"WIN-INVOKER01", []string{"mG04GyLcSJfw", "yF5XiMW1AZUq", "oO1BcZUdiLjv", "bE5jhMy5rPYh"}},
		{"contester-node-1", []string{"tP9bnCGl2a91", "hB6y1l4uI9HJ", "oX7nzf0sDFMB", "nN0fFCkIT2YF"}},
		{"localhost", []string{"xP7TxjO5z7rn", "qC2KmYp5jIUz", "cG1gsgbrijM0", "vX0RuecXyGjh"}},
	}

	for _, tc := range cases {
		got := generatePasswords(tc.hostname, len(tc.want))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("generatePasswords(%q, %d) = %v, want %v", tc.hostname, len(tc.want), got, tc.want)
		}
	}
}

func TestGeneratePasswordsComplexityAndDeterminism(t *testing.T) {
	pws := generatePasswords("node-alpha", 32)
	if len(pws) != 32 {
		t.Fatalf("len = %d, want 32", len(pws))
	}
	for i, pw := range pws {
		if len(pw) != 12 {
			t.Fatalf("pw[%d] = %q len %d, want 12", i, pw, len(pw))
		}
		if !strings.ContainsAny(pw, pwLower) || !strings.ContainsAny(pw, pwUpper) || !strings.ContainsAny(pw, pwDigit) {
			t.Fatalf("pw[%d] = %q missing lower/upper/digit", i, pw)
		}
		for j := 0; j < len(pw); j++ {
			if !strings.ContainsRune(pwAlphabet, rune(pw[j])) {
				t.Fatalf("pw[%d] = %q has non-alphanumeric char %q", i, pw, pw[j])
			}
		}
	}

	// Prefix is independent of count.
	if got, want := generatePasswords("node-alpha", 4), pws[:4]; !reflect.DeepEqual(got, want) {
		t.Fatalf("prefix mismatch: got %v, want %v", got, want)
	}
	if got := generatePasswords("node-alpha", 0); got != nil {
		t.Fatalf("count 0 = %v, want nil", got)
	}
}
