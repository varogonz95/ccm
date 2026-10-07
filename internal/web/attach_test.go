//go:build !windows

package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/varogonz95/clawsh/internal/api"
)

func attachSetup(t *testing.T) (c *http.Client, base string) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "hosts.toml")
	writeHosts(t, cfg, testHost{"a", startAgent(t, "tok"), "tok"}, testHost{"down", closedURL(t), "tok"})
	_, base = startServer(t, cfg)
	return login(t, base), base
}

func sendResize(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	msg, _ := json.Marshal(api.Control{Type: api.ControlResize, Cols: 100, Rows: 30})
	if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		t.Fatal(err)
	}
}

func TestAttachEchoAndExit(t *testing.T) {
	c, base := attachSetup(t)
	s := createViaWeb(t, c, base, "a")
	conn, _, err := dialAttach(base, "a", s.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sendResize(t, conn)
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte("echo hi-$((40+2))\nexit 3\n"))

	out, code := readUntilExit(t, conn)
	if !strings.Contains(out, "hi-42") || code != 3 {
		t.Fatalf("output %q, code %d; want hi-42 and 3", out, code)
	}
}

func TestAttachReplaysScrollback(t *testing.T) {
	c, base := attachSetup(t)
	s := createViaWeb(t, c, base, "a")
	first, _, err := dialAttach(base, "a", s.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.WriteMessage(websocket.BinaryMessage, []byte("echo marker-$((1+1))\n"))
	time.Sleep(500 * time.Millisecond)
	first.Close()

	late, _, err := dialAttach(base, "a", s.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	defer late.Close()
	_ = late.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, data, err := late.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "marker-2") {
		t.Fatalf("replay missing earlier output: %q", data)
	}
}

func TestEndSessionWhileAttached(t *testing.T) {
	c, base := attachSetup(t)
	s := createViaWeb(t, c, base, "a")
	conn, _, err := dialAttach(base, "a", s.ID, base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	sendResize(t, conn)
	if got := del(t, c, base+"/api/hosts/a/sessions/"+s.ID); got != http.StatusNoContent {
		t.Fatalf("delete: %d", got)
	}
	readUntilExit(t, conn) // fails the test if no exit control arrives
}

func TestAttachForeignOriginRejected(t *testing.T) {
	c, base := attachSetup(t)
	s := createViaWeb(t, c, base, "a")
	for _, origin := range []string{"http://evil.example", ""} {
		_, resp, err := dialAttach(base, "a", s.ID, origin)
		if err == nil || resp == nil || resp.StatusCode != http.StatusForbidden {
			t.Errorf("origin %q: err %v, resp %v; want 403", origin, err, resp)
		}
	}
}

func readClose(t *testing.T, conn *websocket.Conn) *websocket.CloseError {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		var ce *websocket.CloseError
		if errors.As(err, &ce) {
			return ce
		}
		if err != nil {
			t.Fatalf("want a close frame, got %v", err)
		}
	}
}

func TestAttachUnknownSession(t *testing.T) {
	_, base := attachSetup(t)
	conn, _, err := dialAttach(base, "a", "nope", base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if ce := readClose(t, conn); ce.Code != closeNotFound {
		t.Fatalf("close code %d (%q), want %d", ce.Code, ce.Text, closeNotFound)
	}
}

func TestAttachOfflineHost(t *testing.T) {
	_, base := attachSetup(t)
	conn, _, err := dialAttach(base, "down", "x", base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if ce := readClose(t, conn); ce.Code != closeUnreachable {
		t.Fatalf("close code %d (%q), want %d", ce.Code, ce.Text, closeUnreachable)
	}
}

func TestAttachWrongToken(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "hosts.toml")
	writeHosts(t, cfg, testHost{"bad", startAgent(t, "tok"), "wrong"})
	_, base := startServer(t, cfg)
	conn, _, err := dialAttach(base, "bad", "x", base)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ce := readClose(t, conn)
	if ce.Code != closeUnreachable || !strings.Contains(ce.Text, "access key rejected") {
		t.Fatalf("close %d %q, want %d with access key rejected", ce.Code, ce.Text, closeUnreachable)
	}
}

func TestAttachUnknownHost(t *testing.T) {
	_, base := attachSetup(t)
	_, resp, err := dialAttach(base, "nope", "x", base)
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("err %v, resp %v; want 404", err, resp)
	}
}
