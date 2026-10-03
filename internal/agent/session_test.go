//go:build !windows

package agent

import (
	"testing"
	"time"

	"ccm/internal/api"
)

func TestResizeAfterExitIsNoop(t *testing.T) {
	m := NewManager(Options{Command: "/bin/sh"})
	defer m.Shutdown()
	s, err := m.Create(api.CreateRequest{Dir: t.TempDir(), Args: []string{"-c", "exit 0"}})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-s.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("session did not finish")
	}
	if err := s.Resize(100, 30); err != nil {
		t.Fatalf("Resize after close: %v, want nil", err)
	}
}
