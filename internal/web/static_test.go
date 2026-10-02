//go:build !windows

package web

import (
	"io"
	"net/http"
	"regexp"
	"testing"
)

// Every file referenced by index.html and app.css must be embedded and served.
func TestStaticAssetsServed(t *testing.T) {
	_, base := startServer(t, noHosts(t))
	c := login(t, base)

	read := func(path string) string {
		resp, err := c.Get(base + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %s", path, resp.Status)
		}
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	index := read("/")
	refs := regexp.MustCompile(`(?:src|href)="(/static/[^"]+)"`).FindAllStringSubmatch(index, -1)
	if len(refs) < 5 {
		t.Fatalf("index.html references only %d static files", len(refs))
	}
	for _, m := range refs {
		read(m[1])
	}
	css := read("/static/app.css")
	for _, m := range regexp.MustCompile(`url\('([^']+)'\)`).FindAllStringSubmatch(css, -1) {
		read("/static/" + m[1])
	}
}
