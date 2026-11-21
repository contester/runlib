package service

import (
	"runtime/debug"
	"testing"

	"github.com/contester/runlib/contester_proto"
)

func TestFormatBuildVersion(t *testing.T) {
	fullSHA := "6abfdb9aebe4bb8bb5d19eb403c2001c50b745a1"
	cases := []struct {
		name    string
		version string
		buildID string
		want    string
	}{
		{
			name:    "short SHA and numeric build ID",
			version: "20261121-193000-6abfdb9",
			buildID: "42",
			want:    "20261121-193000-6abfdb9 #42",
		},
		{
			name:    "short SHA and local build ID",
			version: "20261121-193000-6abfdb9",
			buildID: "local",
			want:    "20261121-193000-6abfdb9 (local)",
		},
		{
			name:    "full 40-char SHA with dirty and empty build ID",
			version: "20261121-193000-" + fullSHA + "-dirty",
			buildID: "",
			want:    "20261121-193000-6abfdb9-dirty",
		},
		{
			name:    "bare 40-char SHA",
			version: fullSHA,
			buildID: "7",
			want:    "6abfdb9 #7",
		},
		{
			name:    "trimmed whitespace",
			version: " 20261121-193000-6abfdb9 ",
			buildID: " 99 ",
			want:    "20261121-193000-6abfdb9 #99",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatBuildVersion(tc.version, tc.buildID); got != tc.want {
				t.Fatalf("FormatBuildVersion(%q, %q) = %q, want %q", tc.version, tc.buildID, got, tc.want)
			}
		})
	}
}

func TestVersionFromBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{
			name: "nil build info",
			info: nil,
			want: "dev",
		},
		{
			name: "vcs time revision and modified",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "6abfdb9aebe4bb8bb5d19eb403c2001c50b745a1"},
					{Key: "vcs.time", Value: "2026-11-21T16:52:18Z"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			want: "20261121-165218-6abfdb9-dirty",
		},
		{
			name: "revision only",
			info: &debug.BuildInfo{
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "6abfdb9aebe4bb8bb5d19eb403c2001c50b745a1"},
				},
			},
			want: "6abfdb9",
		},
		{
			name: "module version fallback",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v1.2.3"},
			},
			want: "v1.2.3",
		},
		{
			name: "devel module version fallback",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
			},
			want: "dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionFromBuildInfo(tc.info); got != tc.want {
				t.Fatalf("versionFromBuildInfo() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIdentifyPopulatesLanguageAndVersion(t *testing.T) {
	c := &Contester{
		InvokerId:      "win-invoker-01",
		Platform:       "win32",
		InvokerVersion: "20261121-193000-6abfdb9 #42",
	}
	var resp contester_proto.IdentifyResponse
	if err := c.Identify(&contester_proto.IdentifyRequest{ContesterId: "director"}, &resp); err != nil {
		t.Fatalf("Identify: %v", err)
	}
	if resp.GetInvokerLanguage() != "go" {
		t.Fatalf("GetInvokerLanguage() = %q, want %q", resp.GetInvokerLanguage(), "go")
	}
	if resp.GetInvokerVersion() != "20261121-193000-6abfdb9 #42" {
		t.Fatalf("GetInvokerVersion() = %q, want explicit version", resp.GetInvokerVersion())
	}

	cDefault := &Contester{InvokerId: "win-invoker-02"}
	var respDefault contester_proto.IdentifyResponse
	if err := cDefault.Identify(&contester_proto.IdentifyRequest{ContesterId: "director"}, &respDefault); err != nil {
		t.Fatalf("Identify default: %v", err)
	}
	if respDefault.GetInvokerLanguage() != "go" {
		t.Fatalf("default GetInvokerLanguage() = %q, want %q", respDefault.GetInvokerLanguage(), "go")
	}
	if respDefault.GetInvokerVersion() == "" {
		t.Fatal("default GetInvokerVersion() is empty")
	}
}
