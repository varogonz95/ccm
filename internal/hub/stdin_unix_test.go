//go:build !windows

package hub

import (
	"errors"
	"os"
	"testing"
	"time"
)

func newPipeReader(t *testing.T) (*cancelReader, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := newCancelReader(r)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		in.Close()
		r.Close()
		w.Close()
	})
	return in, w
}

// TestPumpInputCancel checks that pumpInput blocked with no input returns
// once cancelled, which is what lets Attach return without leaving stdin
// claimed.
func TestPumpInputCancel(t *testing.T) {
	in, w := newPipeReader(t)

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

	in.Cancel()
	select {
	case detached := <-done:
		if detached {
			t.Fatal("cancel reported as detach")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pumpInput still blocked after Cancel")
	}
}

func TestCancelReaderCanceledBeforeRead(t *testing.T) {
	in, w := newPipeReader(t)
	in.Cancel()
	in.Cancel() // idempotent
	w.Write([]byte("x"))
	if _, err := in.Read(make([]byte, 8)); !errors.Is(err, errCanceled) {
		t.Fatalf("Read after Cancel: err = %v, want errCanceled", err)
	}
}
