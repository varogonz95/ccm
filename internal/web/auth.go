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

const cookieName = "ccm_web"

const (
	msgNoCookie  = "Open the link printed by ccm web in your terminal."
	msgBadSecret = "This link is out of date. Open the link printed by ccm web in your terminal."
	msgBadHost   = "This address isn't allowed. Open the link printed by ccm web in your terminal."
	msgBadOrigin = "Requests from other sites are not allowed."
)

// NewSecret returns the per-launch secret for the URL that ccm web opens.
func NewSecret() string { return randHex(32) }

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand does not fail on supported platforms
	}
	return hex.EncodeToString(b)
}

// index exchanges ?k=<secret> for the session cookie, or serves the app.
func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	k := r.URL.Query().Get("k")
	if k == "" {
		s.page(http.HandlerFunc(s.serveIndex)).ServeHTTP(w, r)
		return
	}
	if subtle.ConstantTimeCompare([]byte(k), []byte(s.secret)) != 1 {
		deny(w, r, http.StatusUnauthorized, msgBadSecret)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: s.cookie, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
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

func (s *Server) hasCookie(r *http.Request) bool {
	c, err := r.Cookie(cookieName)
	return err == nil && subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.cookie)) == 1
}

// page guards the HTML and static routes.
func (s *Server) page(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasCookie(r) {
			deny(w, r, http.StatusUnauthorized, msgNoCookie)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// api guards /api routes: the cookie, plus a same-origin Origin whenever the
// browser sends one. WebSocket upgrades must carry Origin; the upgrader
// checks that separately.
func (s *Server) api(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasCookie(r) {
			deny(w, r, http.StatusUnauthorized, msgNoCookie)
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
<title>ccm</title>
<body style="margin:0;padding:48px 24px;background:#0f161d;color:#e6edf3;font:16px/1.5 system-ui,sans-serif">
<h1 style="font-size:28px;margin:0 0 12px">Can't open ccm here</h1>
<p style="margin:0;color:#9fb0bf">%s</p>
</body>
`
