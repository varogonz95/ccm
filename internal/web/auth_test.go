//go:build !windows

package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func noHosts(t *testing.T) string { return filepath.Join(t.TempDir(), "hosts.toml") }

// The page and its assets hold no secrets; only /api needs the key.
func TestPageAndAssetsArePublic(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	for _, p := range []string{"/", "/?k=anything", "/static/app.js"} {
		if got := status(t, http.DefaultClient, base+p); got != http.StatusOK {
			t.Errorf("%s: %d, want 200", p, got)
		}
	}
}

func TestAPIRequiresKey(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	cases := []struct {
		name, path, auth string
	}{
		{"no key", "/api/events", ""},
		{"wrong bearer", "/api/events", "Bearer nope"},
		{"wrong query", "/api/events?k=nope", ""},
		{"key without Bearer", "/api/events", testSecret},
	}
	for _, tc := range cases {
		req, _ := http.NewRequest(http.MethodGet, base+tc.path, nil)
		if tc.auth != "" {
			req.Header.Set("Authorization", tc.auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: %d, want 401", tc.name, resp.StatusCode)
		}
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			t.Errorf("%s: API denial should be JSON, got %q", tc.name, resp.Header.Get("Content-Type"))
		}
	}
}

func TestDeniedPageExplains(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	req, _ := http.NewRequest(http.MethodGet, base+"/", nil)
	req.Host = "evil.example"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusForbidden || !strings.HasPrefix(ct, "text/html") || !strings.Contains(string(body), "clawsh web") {
		t.Fatalf("denied page: %d %s %q", resp.StatusCode, ct, body)
	}
}

func TestForeignHostRejected(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	port := base[strings.LastIndex(base, ":")+1:]
	cases := map[string]int{
		"evil.example:" + port: http.StatusForbidden, // DNS rebinding
		"127.0.0.1:1":          http.StatusForbidden,
		"127.0.0.1":            http.StatusForbidden,
		"localhost":            http.StatusForbidden,
		"localhost.:" + port:   http.StatusForbidden,
		"LOCALHOST:" + port:    http.StatusForbidden,
		"localhost:" + port:    http.StatusOK,
		"[::1]:" + port:        http.StatusOK,
	}
	for host, want := range cases {
		req, _ := http.NewRequest(http.MethodGet, base+"/", nil)
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Host %s: %d, want %d", host, resp.StatusCode, want)
		}
	}
}

func TestAPIGuard(t *testing.T) {
	s, _ := startServer(t, noHosts(t))
	h := s.api(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	port := strconv.Itoa(s.port)
	cases := []struct {
		name   string
		key    string // "header", "query", "wrong" or "" (none)
		origin string
		want   int
	}{
		{"no key", "", "", http.StatusUnauthorized},
		{"wrong key", "wrong", "", http.StatusUnauthorized},
		{"key in header", "header", "", http.StatusNoContent},
		{"key in query", "query", "", http.StatusNoContent},
		{"own origin", "header", "http://127.0.0.1:" + port, http.StatusNoContent},
		{"localhost origin", "header", "http://localhost:" + port, http.StatusNoContent},
		{"foreign origin", "header", "http://evil.example", http.StatusForbidden},
		{"own host, other port", "header", fmt.Sprintf("http://127.0.0.1:%d", s.port+1), http.StatusForbidden},
		{"null origin", "header", "null", http.StatusForbidden},
		{"https origin", "header", "https://127.0.0.1:" + port, http.StatusForbidden},
	}
	for _, tc := range cases {
		target := "/api/x"
		if tc.key == "query" {
			target += "?k=" + testSecret
		}
		req := httptest.NewRequest(http.MethodPost, target, nil)
		switch tc.key {
		case "header":
			req.Header.Set("Authorization", "Bearer "+testSecret)
		case "wrong":
			req.Header.Set("Authorization", "Bearer nope")
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != tc.want {
			t.Errorf("%s: %d, want %d", tc.name, rec.Code, tc.want)
		}
		if tc.want >= 400 && !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("%s: API denial should be JSON, got %q", tc.name, rec.Header().Get("Content-Type"))
		}
	}
}

func TestNewConfigErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := New(Options{ConfigPath: filepath.Join(dir, "missing.toml"), Secret: "x"}); err != nil {
		t.Fatalf("missing hosts file should be fine: %v", err)
	}
	if _, err := New(Options{ConfigPath: filepath.Join(dir, "missing.toml")}); err == nil {
		t.Fatal("expected an error for an empty access key")
	}
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("[[host]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{ConfigPath: bad, Secret: "x"}); err == nil {
		t.Fatal("expected a parse error for a broken hosts file")
	}
}
