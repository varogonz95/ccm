package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

func TestTruncateUTF8(t *testing.T) {
	s := strings.Repeat("a", maxCloseReason-1) + "é" + "tail" // é straddles the limit
	got := truncateUTF8(s, maxCloseReason)
	if !utf8.ValidString(got) || len(got) > maxCloseReason {
		t.Fatalf("got %d bytes, valid=%v", len(got), utf8.ValidString(got))
	}
	if got != strings.Repeat("a", maxCloseReason-1) {
		t.Fatalf("cut in the wrong place: %q", got)
	}
	if truncateUTF8("short", maxCloseReason) != "short" {
		t.Fatal("short string changed")
	}
}

// wsPair returns the client end of a WebSocket whose server end is passed to
// serve.
func wsPair(t *testing.T, serve func(*websocket.Conn)) *websocket.Conn {
	t.Helper()
	up := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		serve(c)
	}))
	t.Cleanup(srv.Close)
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// A source that vanishes without a close frame (1006 locally) must reach the
// other side as the given code, never as 1006, which is not allowed on the wire.
func TestPipeReplacesAbnormalClose(t *testing.T) {
	for _, lost := range []int{closeUnreachable, websocket.CloseNormalClosure} {
		src := wsPair(t, func(c *websocket.Conn) { c.NetConn().Close() })

		got := make(chan int, 1)
		dst := wsPair(t, func(c *websocket.Conn) {
			_, _, err := c.ReadMessage()
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				got <- ce.Code
			} else {
				got <- -1
			}
			c.Close()
		})

		pipe(dst, src, lost)
		select {
		case code := <-got:
			if code != lost {
				t.Errorf("close code = %d, want %d", code, lost)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("no close frame")
		}
	}
}

func TestPipeForwardsCloseCode(t *testing.T) {
	src := wsPair(t, func(c *websocket.Conn) {
		_ = c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4404, "gone"))
		c.Close()
	})
	got := make(chan string, 1)
	dst := wsPair(t, func(c *websocket.Conn) {
		_, _, err := c.ReadMessage()
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			got <- ce.Error()
		}
		c.Close()
	})
	pipe(dst, src, closeUnreachable)
	select {
	case s := <-got:
		if !strings.Contains(s, "4404") || !strings.Contains(s, "gone") {
			t.Errorf("got %q, want 4404 gone", s)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no close frame")
	}
}
