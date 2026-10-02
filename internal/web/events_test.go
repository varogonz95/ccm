//go:build !windows

package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ccm/internal/api"
	"ccm/internal/hub"
)

func TestOverviewOnlineAndOffline(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "hosts.toml")
	writeHosts(t, cfg, testHost{"up", startAgent(t, "tok"), "tok"}, testHost{"down", closedURL(t), "tok"})
	_, base := startServer(t, cfg)

	ov := next(t, subscribe(t, login(t, base), base))

	if ov.ConfigPath != cfg {
		t.Errorf("config_path = %q, want %q", ov.ConfigPath, cfg)
	}
	if len(ov.Hosts) != 2 || !ov.Hosts[0].Online || ov.Hosts[1].Online || ov.Hosts[1].Error == "" {
		t.Fatalf("hosts = %+v", ov.Hosts)
	}
}

func TestOverviewPushesOnlyChanges(t *testing.T) {
	agentURL := startAgent(t, "tok")
	cfg := filepath.Join(t.TempDir(), "hosts.toml")
	writeHosts(t, cfg, testHost{"a", agentURL, "tok"})
	_, base := startServer(t, cfg)
	events := subscribe(t, login(t, base), base)

	if n := len(next(t, events).Hosts[0].Sessions); n != 0 {
		t.Fatalf("sessions = %d, want 0", n)
	}
	select {
	case ov := <-events:
		t.Fatalf("event without a change: %+v", ov)
	case <-time.After(300 * time.Millisecond): // six poll intervals
	}

	c := hub.NewClient(hub.Host{Name: "a", URL: agentURL, Token: "tok"})
	if _, err := c.Create(context.Background(), api.CreateRequest{Dir: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if n := len(next(t, events).Hosts[0].Sessions); n != 1 {
		t.Fatalf("sessions = %d, want 1", n)
	}
}

func TestHostsFileReload(t *testing.T) {
	agentURL := startAgent(t, "tok")
	cfg := filepath.Join(t.TempDir(), "hosts.toml")
	writeHosts(t, cfg, testHost{"a", agentURL, "tok"})
	_, base := startServer(t, cfg)
	events := subscribe(t, login(t, base), base)
	if n := len(next(t, events).Hosts); n != 1 {
		t.Fatalf("hosts = %d, want 1", n)
	}

	writeHosts(t, cfg, testHost{"a", agentURL, "tok"}, testHost{"b", agentURL, "tok"})
	if n := len(next(t, events).Hosts); n != 2 {
		t.Fatalf("after adding: hosts = %d, want 2", n)
	}

	if err := os.WriteFile(cfg, []byte("[[host]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bumpMtime(t, cfg)
	select {
	case ov := <-events:
		t.Fatalf("broken file must keep the last good hosts, got %+v", ov.Hosts)
	case <-time.After(300 * time.Millisecond):
	}

	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}
	if n := len(next(t, events).Hosts); n != 0 {
		t.Fatalf("after deleting the file: hosts = %d, want 0 (first-run screen)", n)
	}
}

func TestEventsWithoutKey(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	if got := status(t, http.DefaultClient, base+"/api/events"); got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", got)
	}
}

func TestSlowSubscriberDropped(t *testing.T) {
	hosts, err := loadHostsFile(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil {
		t.Fatal(err)
	}
	b := newBroadcaster(hosts, time.Hour)
	ch := b.subscribe()
	defer b.unsubscribe(ch)

	b.mu.Lock()
	for i := 0; i < subBuffer+2; i++ {
		b.sendLocked([]byte(fmt.Sprint(i)))
	}
	b.mu.Unlock()

	n := 0
	for range ch { // ends only if the subscriber was dropped (channel closed)
		n++
	}
	if n > subBuffer {
		t.Fatalf("received %d events, buffer is %d", n, subBuffer)
	}
	b.mu.Lock()
	stopped := b.cancel == nil
	b.mu.Unlock()
	if !stopped {
		t.Fatal("poller still running after the last subscriber was dropped")
	}
}
