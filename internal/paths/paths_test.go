package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir(t *testing.T) {
	base := t.TempDir()
	if got, want := configDir(base), filepath.Join(base, "clawsh"); got != want {
		t.Fatalf("fresh install: got %s, want %s", got, want)
	}

	legacy := filepath.Join(base, "ccm")
	if err := os.Mkdir(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := configDir(base); got != legacy {
		t.Fatalf("legacy only: got %s, want %s", got, legacy)
	}

	current := filepath.Join(base, "clawsh")
	if err := os.Mkdir(current, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := configDir(base); got != current {
		t.Fatalf("both present: got %s, want %s", got, current)
	}
}
