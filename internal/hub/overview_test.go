//go:build !windows

package hub_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ccm/internal/agent"
	"ccm/internal/api"
	"ccm/internal/hub"
)

func startAgent(t *testing.T, token string) string {
	t.Helper()
	m := agent.NewManager(agent.Options{Command: "/bin/sh"})
	srv := httptest.NewServer(agent.NewServer(m, token).Handler())
	t.Cleanup(func() { srv.Close(); m.Shutdown() })
	return srv.URL
}

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

func TestOverviewReportsEachHostInOrder(t *testing.T) {
	up := startAgent(t, "tok")
	c := hub.NewClient(hub.Host{Name: "up", URL: up, Token: "tok"})
	if _, err := c.Create(context.Background(), api.CreateRequest{Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	hosts := []hub.Host{
		{Name: "up", URL: up, Token: "tok"},
		{Name: "down", URL: closedURL(t), Token: "tok"},
		{Name: "badkey", URL: up, Token: "wrong"},
	}

	got := hub.Overview(context.Background(), hosts)

	if len(got) != 3 || got[0].Name != "up" || got[1].Name != "down" || got[2].Name != "badkey" {
		t.Fatalf("order/len wrong: %+v", got)
	}
	if !got[0].Online || got[0].Health == nil || len(got[0].Sessions) != 1 || got[0].Error != "" {
		t.Errorf("up: %+v", got[0])
	}
	if got[1].Online || got[1].Health != nil || got[1].Error == "" || got[1].Sessions == nil {
		t.Errorf("down: %+v (Sessions must be empty, not nil)", got[1])
	}
	if got[2].Online || got[2].Health == nil || got[2].Error != hub.ErrMsgAuth {
		t.Errorf("badkey: %+v", got[2])
	}
}

func TestOverviewTimesOutSlowHost(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	defer slow.Close()

	start := time.Now()
	got := hub.Overview(context.Background(), []hub.Host{{Name: "slow", URL: slow.URL, Token: "x"}})
	if took := time.Since(start); took > hub.OverviewTimeout+time.Second {
		t.Fatalf("took %v, want about %v", took, hub.OverviewTimeout)
	}
	if got[0].Online || got[0].Error == "" {
		t.Fatalf("slow host: %+v", got[0])
	}
}

func TestHTTPErrorCarriesStatus(t *testing.T) {
	up := startAgent(t, "tok")
	err := hub.NewClient(hub.Host{Name: "up", URL: up, Token: "tok"}).Kill(context.Background(), "nope")
	he, ok := err.(*hub.HTTPError)
	if !ok {
		t.Fatalf("err = %T %v, want *hub.HTTPError", err, err)
	}
	if he.Code != http.StatusNotFound || he.Msg != "session not found" {
		t.Fatalf("got %+v", he)
	}
	if he.Error() != "404 Not Found: session not found" {
		t.Fatalf("Error() = %q", he.Error())
	}
}
