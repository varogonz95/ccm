package api

import (
	"runtime/debug"
	"testing"
)

func TestDevVersion(t *testing.T) {
	rev := debug.BuildSetting{Key: "vcs.revision", Value: "0123456789abcdef0123"}
	for _, c := range []struct {
		name string
		in   []debug.BuildSetting
		want string
	}{
		{"no vcs", nil, "dev"},
		{"clean", []debug.BuildSetting{rev, {Key: "vcs.modified", Value: "false"}}, "dev+0123456789ab"},
		{"dirty", []debug.BuildSetting{rev, {Key: "vcs.modified", Value: "true"}}, "dev+0123456789ab.dirty"},
	} {
		if got := devVersion(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestDisplayVersion(t *testing.T) {
	for in, want := range map[string]string{
		"0.1.0":            "v0.1.0",
		"0.2.0-rc.1":       "v0.2.0-rc.1",
		"dev":              "dev",
		"dev+0123abcd":     "dev+0123abcd",
		"0.1.0-3-gabc1234": "v0.1.0-3-gabc1234",
		"":                 "",
	} {
		if got := DisplayVersion(in); got != want {
			t.Errorf("DisplayVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
