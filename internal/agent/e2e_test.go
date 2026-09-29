//go:build !windows

package agent_test

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"ccm/internal/agent"
	"ccm/internal/api"
	"ccm/internal/hub"
)

// End-to-end over real HTTP/WebSocket, with /bin/sh standing in for claude.
func TestCreateAttachEchoExit(t *testing.T) {
	m := agent.NewManager(agent.Options{Command: "/bin/sh"})
	defer m.Shutdown()
	srv := httptest.NewServer(agent.NewServer(m, "secret").Handler())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bad := hub.NewClient(hub.Host{Name: "t", URL: srv.URL, Token: "wrong"})
	if _, err := bad.List(ctx); err == nil {
		t.Fatal("expected auth failure with wrong token")
	}

	c := hub.NewClient(hub.Host{Name: "t", URL: srv.URL, Token: "secret"})
	s, err := c.Create(ctx, api.CreateRequest{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := c.Dial(ctx, s.ID[:4]) // prefix lookup
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	resize, _ := json.Marshal(api.Control{Type: api.ControlResize, Cols: 100, Rows: 30})
	_ = conn.WriteMessage(websocket.TextMessage, resize)
	_ = conn.WriteMessage(websocket.BinaryMessage, []byte("echo hello-$((40+2))\nexit 3\n"))

	var out strings.Builder
	var code *int
	_ = conn.SetReadDeadline(time.Now().Add(8 * time.Second))
	for code == nil {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read: %v (output so far: %q)", err, out.String())
		}
		if mt == websocket.BinaryMessage {
			out.Write(data)
			continue
		}
		var ctl api.Control
		if json.Unmarshal(data, &ctl) == nil && ctl.Type == api.ControlExit {
			code = ctl.Code
		}
	}
	if !strings.Contains(out.String(), "hello-42") {
		t.Fatalf("missing command output, got %q", out.String())
	}
	if *code != 3 {
		t.Fatalf("exit code = %d, want 3", *code)
	}

	list, err := c.List(ctx)
	if err != nil || len(list) != 1 || list[0].Status != api.StatusExited {
		t.Fatalf("list after exit = %+v, err %v", list, err)
	}
	if err := c.Kill(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := c.List(ctx); len(list) != 0 {
		t.Fatalf("session not removed: %+v", list)
	}
}

// A second viewer attaching later must get the earlier output replayed.
func TestReplayOnLateAttach(t *testing.T) {
	m := agent.NewManager(agent.Options{Command: "/bin/sh"})
	defer m.Shutdown()
	srv := httptest.NewServer(agent.NewServer(m, "k").Handler())
	defer srv.Close()
	ctx := context.Background()
	c := hub.NewClient(hub.Host{Name: "t", URL: srv.URL, Token: "k"})

	s, err := c.Create(ctx, api.CreateRequest{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	first, err := c.Dial(ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	_ = first.WriteMessage(websocket.BinaryMessage, []byte("echo marker-$((1+1))\n"))
	time.Sleep(500 * time.Millisecond)
	first.Close()

	late, err := c.Dial(ctx, s.ID)
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
