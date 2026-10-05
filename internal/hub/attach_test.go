package hub

import (
	"os"
	"sync"
	"testing"
)

type sink struct {
	mu  sync.Mutex
	got []byte
}

func (s *sink) send(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, b...)
	return nil
}

func (s *sink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return string(s.got)
}

func TestPumpInputDetach(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	var s sink
	w.Write([]byte("ls\x1dignored"))
	if !pumpInput(r, s.send) {
		t.Fatal("pumpInput did not report detach")
	}
	if s.String() != "ls" {
		t.Fatalf("sent %q, want %q", s.String(), "ls")
	}
}
