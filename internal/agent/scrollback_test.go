package agent

import (
	"bytes"
	"testing"
)

func TestScrollbackKeepsTail(t *testing.T) {
	s := NewScrollback(4)
	for _, c := range []string{"ab", "cd", "ef", "gh", "ij"} {
		s.Write([]byte(c))
	}
	if got := s.Snapshot(); !bytes.Equal(got, []byte("ghij")) {
		t.Fatalf("got %q, want %q", got, "ghij")
	}
}
