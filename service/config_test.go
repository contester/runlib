package service

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfigDecodesNoUserProfile(t *testing.T) {
	var c contesterConfig
	if _, err := toml.Decode("no_user_profile = true\n", &c); err != nil {
		t.Fatal(err)
	}
	if !c.NoUserProfile {
		t.Fatal("no_user_profile = true did not decode into contesterConfig.NoUserProfile")
	}
}

func TestConfigNoUserProfileDefaultsOff(t *testing.T) {
	var c contesterConfig
	if _, err := toml.Decode("path = \"x\"\n", &c); err != nil {
		t.Fatal(err)
	}
	if c.NoUserProfile {
		t.Fatal("NoUserProfile defaulted to true; must be off unless no_user_profile is set")
	}
}
