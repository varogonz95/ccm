//go:build !windows

package hub

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// TestAttachReturnsWhenSendStuck: an agent that stops reading (host gone
// without a RST) leaves the input goroutine blocked in a websocket write.
// Cancelling ctx must still make Attach return.
func TestAttachReturnsWhenSendStuck(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		<-release // never read
	}))
	defer srv.Close()
	defer close(release)

	ptm, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty:", err)
	}
	defer ptm.Close()
	defer tty.Close()
	go io.Copy(io.Discard, ptm)

	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = tty, tty
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- Attach(ctx, NewClient(Host{Name: "t", URL: srv.URL}), "x") }()

	// Type until input stops draining: the websocket write is then blocked.
	var typed atomic.Int64
	stopTyping := make(chan struct{})
	defer close(stopTyping)
	go func() {
		chunk := bytes.Repeat([]byte("a"), 4096)
		for {
			select {
			case <-stopTyping:
				return
			default:
			}
			n, err := ptm.Write(chunk)
			if err != nil {
				return
			}
			typed.Add(int64(n))
		}
	}()
	last, stalled := int64(-1), time.Time{}
	for deadline := time.Now().Add(20 * time.Second); ; {
		if time.Now().After(deadline) {
			t.Fatal("input never stalled")
		}
		if n := typed.Load(); n != last {
			last, stalled = n, time.Now()
		} else if time.Since(stalled) > 500*time.Millisecond {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errc:
		if err != context.Canceled {
			t.Fatalf("Attach: %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Attach did not return after cancel while a send was stuck")
	}
}
