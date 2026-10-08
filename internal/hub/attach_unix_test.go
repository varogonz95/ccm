//go:build !windows

package hub

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

// smallBuffer pins both ends of the fake agent's TCP connection to small
// socket buffers. Left alone, Linux autotunes them up to megabytes, so how
// long a sender takes to stall against a peer that stops reading depends on
// how fast the machine is; on a slow CI runner it can exceed the tests'
// deadlines. With fixed buffers it stalls after a few KB everywhere.
const smallBuffer = 8 << 10

type smallBufferListener struct{ net.Listener }

func (l smallBufferListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetReadBuffer(smallBuffer)
	}
	return c, err
}

// fakeAgent serves the attach websocket with handle; the handler returns
// when the test ends.
func fakeAgent(t *testing.T, handle func(*websocket.Conn, <-chan struct{})) *Client {
	t.Helper()
	release := make(chan struct{})
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		handle(conn, release)
	}))
	srv.Listener = smallBufferListener{srv.Listener}
	srv.Start()
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	// Client uses websocket.DefaultDialer; hub tests don't run in parallel.
	d := *websocket.DefaultDialer
	d.NetDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if tc, ok := c.(*net.TCPConn); ok {
			tc.SetWriteBuffer(smallBuffer)
		}
		return c, err
	}
	old := websocket.DefaultDialer
	websocket.DefaultDialer = &d
	t.Cleanup(func() { websocket.DefaultDialer = old })

	return NewClient(Host{Name: "t", URL: srv.URL})
}

// neverRead models an agent host that vanished without a RST.
func neverRead(_ *websocket.Conn, release <-chan struct{}) { <-release }

type ptyTerm struct {
	ptm, tty *os.File
	mu       sync.Mutex
	out      bytes.Buffer // everything attach wrote to the terminal
}

func newPTY(t *testing.T) *ptyTerm {
	t.Helper()
	ptm, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty:", err)
	}
	p := &ptyTerm{ptm: ptm, tty: tty}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptm.Read(buf)
			p.mu.Lock()
			p.out.Write(buf[:n])
			p.mu.Unlock()
			if err != nil {
				return
			}
		}
	}()
	t.Cleanup(func() {
		tty.Close()
		ptm.Close()
	})
	return p
}

func (p *ptyTerm) output() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.out.String()
}

// typeUntilStalled types until input stops draining, i.e. a websocket write
// is blocked, then keeps the typing goroutine going until the test ends.
func (p *ptyTerm) typeUntilStalled(t *testing.T) {
	t.Helper()
	var typed atomic.Int64
	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	go func() {
		chunk := bytes.Repeat([]byte("a"), 4096)
		for {
			select {
			case <-stop:
				return
			default:
			}
			n, err := p.ptm.Write(chunk)
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
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func waitAttach(t *testing.T, errc <-chan error, within time.Duration) error {
	t.Helper()
	select {
	case err := <-errc:
		return err
	case <-time.After(within):
		t.Fatal("attach did not return")
		return nil
	}
}

// A send stuck on a dead connection must not keep ctx cancel from ending
// attach.
func TestAttachCancelWhileSendStuck(t *testing.T) {
	c := fakeAgent(t, neverRead)
	p := newPTY(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- attach(ctx, c, "x", p.tty, p.tty) }()

	p.typeUntilStalled(t)
	cancel()
	if err := waitAttach(t, errc, 3*time.Second); err != context.Canceled {
		t.Fatalf("attach: %v, want context.Canceled", err)
	}
}

// The CLI never cancels ctx, so a dead connection must end attach by itself.
func TestAttachDeadConnection(t *testing.T) {
	old := writeTimeout
	writeTimeout = 300 * time.Millisecond
	defer func() { writeTimeout = old }()

	c := fakeAgent(t, neverRead)
	p := newPTY(t)
	errc := make(chan error, 1)
	go func() { errc <- attach(context.Background(), c, "x", p.tty, p.tty) }()

	// Stalls once the socket buffers fill, then the write deadline fires.
	go func() {
		chunk := bytes.Repeat([]byte("a"), 4096)
		for {
			if _, err := p.ptm.Write(chunk); err != nil {
				return
			}
		}
	}()
	err := waitAttach(t, errc, 20*time.Second)
	if err == nil || !strings.Contains(err.Error(), "connection lost") {
		t.Fatalf("attach: %v, want connection lost", err)
	}
}

// Nothing from the session may reach the terminal after the detach line, or
// it would land on whatever the caller draws next.
func TestAttachDetachLeavesNoOutput(t *testing.T) {
	c := fakeAgent(t, func(conn *websocket.Conn, release <-chan struct{}) {
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		for {
			select {
			case <-release:
				return
			default:
			}
			if conn.WriteMessage(websocket.BinaryMessage, []byte("out ")) != nil {
				return
			}
		}
	})
	p := newPTY(t)
	errc := make(chan error, 1)
	go func() { errc <- attach(context.Background(), c, "x", p.tty, p.tty) }()

	for deadline := time.Now().Add(5 * time.Second); !strings.Contains(p.output(), "out "); {
		if time.Now().After(deadline) {
			t.Fatal("no session output")
		}
		time.Sleep(10 * time.Millisecond)
	}
	p.ptm.Write([]byte{DetachKey})
	if err := waitAttach(t, errc, 3*time.Second); err != nil {
		t.Fatalf("attach: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	// The restored tty maps \n to \r\n; ignore carriage returns.
	if out := strings.ReplaceAll(p.output(), "\r", ""); !strings.HasSuffix(out, "[detached from t/x]\n") {
		t.Fatalf("terminal output does not end with the detach line: %q", out[max(0, len(out)-80):])
	}
}
