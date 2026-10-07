package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	msgNoKey     = "Open the link printed by clawsh web in your terminal."
	msgBadHost   = "This address isn't allowed. Open the link printed by clawsh web in your terminal."
	msgBadOrigin = "Requests from other sites are not allowed."
)

// NewSecret returns the per-launch access key. The URL that clawsh web opens
// carries it to the page, which keeps it in localStorage and sends it with
// every API request.
//
// It is deliberately not a cookie: browsers send a host's cookies to every
// port on it, so another local user's server on 127.0.0.1:<other port> could
// collect one. localStorage is scoped to the exact origin, port included.
func NewSecret() string { return randHex(32) }

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return hex.EncodeToString(b)
}

// checkHost rejects requests whose Host is not this server on loopback.
// This stops DNS rebinding: a hostile page that points its own domain at
// 127.0.0.1 still sends its own domain in Host.
func (s *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.ownHost(r.Host) {
			deny(w, r, http.StatusForbidden, msgBadHost)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) ownHost(hostport string) bool {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil || port != strconv.Itoa(s.port) {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ownOrigin reports whether an Origin header names this server.
func (s *Server) ownOrigin(origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Scheme == "http" && s.ownHost(u.Host)
}

// keyFrom returns the access key from "Authorization: Bearer <key>", or from
// ?k=<key> for EventSource and WebSocket, which can't set headers.
func keyFrom(r *http.Request) string {
	if k, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		return k
	}
	return r.URL.Query().Get("k")
}

func (s *Server) hasKey(r *http.Request) bool {
	k := keyFrom(r)
	return k != "" && subtle.ConstantTimeCompare([]byte(k), []byte(s.secret)) == 1
}

// api guards /api routes: the access key, plus a same-origin Origin whenever
// the browser sends one. WebSocket upgrades must carry Origin; the upgrader
// checks that separately.
func (s *Server) api(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasKey(r) {
			deny(w, r, http.StatusUnauthorized, msgNoKey)
			return
		}
		if o := r.Header.Get("Origin"); o != "" && !s.ownOrigin(o) {
			deny(w, r, http.StatusForbidden, msgBadOrigin)
			return
		}
		next(w, r)
	})
}

// deny explains a refusal: JSON for /api, a short page for everything else.
func deny(w http.ResponseWriter, r *http.Request, code int, msg string) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeErr(w, code, errors.New(msg))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, deniedPage, html.EscapeString(msg))
}

const deniedPage = `<!doctype html>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>clawsh</title>
<body style="margin:0;padding:48px 24px;background:#0f161d;color:#e6edf3;font:16px/1.5 system-ui,sans-serif">
<h1 style="font-size:28px;margin:0 0 12px">Can't open clawsh here</h1>
<p style="margin:0;color:#9fb0bf">%s</p>
</body>
`
