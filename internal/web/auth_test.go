//go:build !windows

package web

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func noHosts(t *testing.T) string { return filepath.Join(t.TempDir(), "hosts.toml") }

func TestSecretExchangeSetsCookie(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(base + "/?k=" + testSecret)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("got %s to %q, want 303 to /", resp.Status, resp.Header.Get("Location"))
	}
	var got *http.Cookie
	for _, ck := range resp.Cookies() {
		if ck.Name == cookieName {
			got = ck
		}
	}
	if got == nil {
		t.Fatal("no session cookie")
	}
	if got.Value == testSecret {
		t.Fatal("cookie must not reuse the URL secret")
	}
	if !got.HttpOnly || got.SameSite != http.SameSiteStrictMode || got.Path != "/" {
		t.Fatalf("cookie flags: %+v", got)
	}
}

func TestWrongSecretRejected(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	if got := status(t, http.DefaultClient, base+"/?k=nope"); got != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", got)
	}
}

func TestPagesNeedCookie(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	paths := []string{"/", "/static/app.js"}
	for _, p := range paths {
		if got := status(t, http.DefaultClient, base+p); got != http.StatusUnauthorized {
			t.Errorf("%s without cookie: %d, want 401", p, got)
		}
	}
	c := login(t, base)
	for _, p := range paths {
		if got := status(t, c, base+p); got != http.StatusOK {
			t.Errorf("%s with cookie: %d, want 200", p, got)
		}
	}
}

func TestDeniedPageExplains(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") || !strings.Contains(string(body), "ccm web") {
		t.Fatalf("denied page: %s %q", ct, body)
	}
}

func TestForeignHostRejected(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	c := login(t, base)
	port := base[strings.LastIndex(base, ":")+1:]
	cases := map[string]int{
		"evil.example:" + port: http.StatusForbidden, // DNS rebinding
		"127.0.0.1:1":          http.StatusForbidden,
		"localhost:" + port:    http.StatusOK,
	}
	for host, want := range cases {
		req, _ := http.NewRequest(http.MethodGet, base+"/", nil)
		req.Host = host
		resp, err := c.Do(req)
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
	own := fmt.Sprintf("http://127.0.0.1:%d", s.port)
	cases := []struct {
		name   string
		cookie bool
		origin string
		want   int
	}{
		{"no cookie", false, "", http.StatusUnauthorized},
		{"cookie, no origin", true, "", http.StatusNoContent},
		{"cookie, own origin", true, own, http.StatusNoContent},
		{"cookie, foreign origin", true, "http://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
		if tc.cookie {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: s.cookie})
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
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("[[host]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{ConfigPath: bad, Secret: "x"}); err == nil {
		t.Fatal("expected a parse error for a broken hosts file")
	}
}
