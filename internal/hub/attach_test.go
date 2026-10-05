//go:build !windows

package hub

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/muesli/cancelreader"
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

// TestPumpInputCancel checks that a reader blocked with no input returns once
// cancelled, which is what lets Attach return without leaving stdin claimed.
func TestPumpInputCancel(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	in, err := cancelreader.NewReader(r)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()

	var s sink
	done := make(chan bool, 1)
	go func() { done <- pumpInput(in, s.send) }()

	w.Write([]byte("hi"))
	deadline := time.Now().Add(2 * time.Second)
	for s.String() != "hi" {
		if time.Now().After(deadline) {
			t.Fatalf("sent %q, want %q", s.String(), "hi")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !in.Cancel() {
		t.Fatal("Cancel not supported for a pipe")
	}
	select {
	case detached := <-done:
		if detached {
			t.Fatal("cancel reported as detach")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pumpInput still blocked after Cancel")
	}
}
