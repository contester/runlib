package service

import (
	"runtime/debug"
	"strings"
	"time"
)

// InvokerLanguage identifies the implementation language of this invoker.
const InvokerLanguage = "go"

// FormatBuildVersion combines injected build version and build ID strings into
// a concise display string, falling back to embedded VCS metadata when version
// is empty.
func FormatBuildVersion(version, buildID string) string {
	base := strings.TrimSpace(version)
	buildID = strings.TrimSpace(buildID)
	if base == "" {
		base = DefaultInvokerVersion()
	} else {
		base = shortenTrailingSHA(base)
	}
	switch buildID {
	case "":
		return base
	case "local":
		return base + " (local)"
	default:
		return base + " #" + buildID
	}
}

// DefaultInvokerVersion returns a build version derived from Go's embedded VCS
// build info, falling back to the module version or "dev".
func DefaultInvokerVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	return versionFromBuildInfo(info)
}

func versionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "dev"
	}
	var rev, timeRaw, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.time":
			timeRaw = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}

	var timeStr string
	if timeRaw != "" {
		if t, err := time.Parse(time.RFC3339, timeRaw); err == nil {
			timeStr = t.UTC().Format("20060102-150405")
		}
	}

	var base string
	switch {
	case timeStr != "" && rev != "":
		base = timeStr + "-" + rev
	case rev != "":
		base = rev
	case info.Main.Version != "" && info.Main.Version != "(devel)":
		base = info.Main.Version
	default:
		base = "dev"
	}

	if rev != "" && modified == "true" {
		base += "-dirty"
	}
	return base
}

func shortenTrailingSHA(v string) string {
	core := v
	dirty := ""
	if strings.HasSuffix(core, "-dirty") {
		core = strings.TrimSuffix(core, "-dirty")
		dirty = "-dirty"
	}
	prefix := ""
	cand := core
	if idx := strings.LastIndexByte(core, '-'); idx >= 0 {
		prefix = core[:idx+1]
		cand = core[idx+1:]
	}
	if len(cand) == 40 && isHexString(cand) {
		return prefix + cand[:7] + dirty
	}
	return v
}

func isHexString(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}
