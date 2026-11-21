package service

import (
	"reflect"
	"testing"
)

func TestGetPasswordsExplicitPasswords(t *testing.T) {
	cfg := contesterConfig{
		Passwords:    "alpha beta gamma",
		SandboxCount: 10,
	}
	want := []string{"alpha", "beta", "gamma"}
	if got := getPasswords(&cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("getPasswords(%+v) = %v, want %v", cfg, got, want)
	}
}

func TestGetPasswordsExplicitSandboxCount(t *testing.T) {
	cfg := contesterConfig{
		SandboxCount: 4,
	}
	got := getPasswords(&cfg)
	want := generatePasswords(getHostname(), 4)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("getPasswords(%+v) = %v, want %v", cfg, got, want)
	}
}
