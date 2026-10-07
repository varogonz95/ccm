//go:build !windows

package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/varogonz95/clawsh/internal/agent"
	"github.com/varogonz95/clawsh/internal/api"
)

const testSecret = "test-secret"

type testHost struct{ name, url, token string }

// startAgent runs a real agent with /bin/sh standing in for claude.
func startAgent(t *testing.T, token string) string {
	t.Helper()
	m := agent.NewManager(agent.Options{Command: "/bin/sh"})
	srv := httptest.NewServer(agent.NewServer(m, token).Handler())
	t.Cleanup(func() { srv.Close(); m.Shutdown() })
	return srv.URL
}

// closedURL is an http URL on which nothing listens.
func closedURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return "http://" + addr
}

func writeHosts(t *testing.T, path string, hosts ...testHost) {
	t.Helper()
	var b strings.Builder
	for _, h := range hosts {
		fmt.Fprintf(&b, "[[host]]\nname = %q\nurl = %q\ntoken = %q\n\n", h.name, h.url, h.token)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	bumpMtime(t, path)
}

var mtimeTick atomic.Int64

// bumpMtime moves the file's mtime forward so a rewrite is always noticed,
// even on filesystems with coarse timestamps.
func bumpMtime(t *testing.T, path string) {
	t.Helper()
	ts := time.Now().Add(time.Duration(mtimeTick.Add(1)) * time.Second)
	if err := os.Chtimes(path, ts, ts); err != nil {
		t.Fatal(err)
	}
}

// startServer runs a web Server on a loopback port and returns it with its base URL.
func startServer(t *testing.T, cfgPath string) (*Server, string) {
	t.Helper()
	ts := httptest.NewUnstartedServer(nil)
	port := ts.Listener.Addr().(*net.TCPAddr).Port
	s, err := New(Options{ConfigPath: cfgPath, Secret: testSecret, Port: port, PollEvery: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ts.Config.Handler = s.Handler()
	ts.Start()
	t.Cleanup(ts.Close)
	return s, ts.URL
}

// keyTransport sends the access key the way the page's fetch calls do.
type keyTransport struct{ key string }

func (k keyTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+k.key)
	return http.DefaultTransport.RoundTrip(r)
}

// login returns a client that sends the access key with every request.
func login(t *testing.T, base string) *http.Client {
	t.Helper()
	return &http.Client{Transport: keyTransport{testSecret}}
}

func status(t *testing.T, c *http.Client, url string) int {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

// subscribe opens /api/events and delivers each overview event on the channel.
func subscribe(t *testing.T, c *http.Client, base string) <-chan api.Overview {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/events", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events: %s", resp.Status)
	}
	t.Cleanup(func() { cancel(); resp.Body.Close() })
	ch := make(chan api.Overview, 16)
	go func() {
		defer close(ch)
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<20), 1<<20)
		for sc.Scan() {
			data, ok := strings.CutPrefix(sc.Text(), "data: ")
			if !ok {
				continue
			}
			var ov api.Overview
			if json.Unmarshal([]byte(data), &ov) == nil {
				ch <- ov
			}
		}
	}()
	return ch
}

func next(t *testing.T, ch <-chan api.Overview) api.Overview {
	t.Helper()
	select {
	case ov, ok := <-ch:
		if !ok {
			t.Fatal("event stream closed")
		}
		return ov
	case <-time.After(5 * time.Second):
		t.Fatal("no overview event within 5s")
	}
	return api.Overview{}
}

func createViaWeb(t *testing.T, c *http.Client, base, host string) api.Session {
	t.Helper()
	body, _ := json.Marshal(api.CreateRequest{Dir: t.TempDir()})
	resp, err := c.Post(base+"/api/hosts/"+host+"/sessions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %s", resp.Status)
	}
	var s api.Session
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// dialAttach opens the web attach WebSocket with the key in the query, as
// the page does (browsers can't set headers on WebSockets). host must
// already be path-escaped.
func dialAttach(base, host, id, origin string) (*websocket.Conn, *http.Response, error) {
	u := "ws" + strings.TrimPrefix(base, "http") + "/api/hosts/" + host + "/sessions/" + id + "/attach?k=" + testSecret
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	return d.Dial(u, hdr)
}

// readUntilExit collects terminal output until the exit control arrives.
func readUntilExit(t *testing.T, conn *websocket.Conn) (string, int) {
	t.Helper()
	var out strings.Builder
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v (output so far: %q)", err, out.String())
		}
		if mt == websocket.BinaryMessage {
			out.Write(data)
			continue
		}
		var ctl api.Control
		if json.Unmarshal(data, &ctl) == nil && ctl.Type == api.ControlExit && ctl.Code != nil {
			return out.String(), *ctl.Code
		}
	}
}
