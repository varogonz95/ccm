package api

import "runtime/debug"

// Version is set at build time: -ldflags "-X github.com/varogonz95/clawsh/internal/api.Version=1.2.3"
// (the Makefile derives it from the latest v* git tag). A plain `go build`
// leaves it at "dev", which init extends with the commit Go records in the
// binary, so differently built agents can still be told apart.
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		Version = devVersion(info.Settings)
	}
}

// devVersion returns "dev+<12-char commit>[.dirty]", or "dev" without VCS info.
func devVersion(settings []debug.BuildSetting) string {
	var rev string
	var dirty bool
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	if dirty {
		return "dev+" + rev + ".dirty"
	}
	return "dev+" + rev
}

// DisplayVersion formats a version for people: release versions get a "v"
// prefix ("v0.1.0"), anything else (dev builds, commit hashes) is shown as is.
func DisplayVersion(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}
