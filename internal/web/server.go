// Package web serves the browser UI for the hub: `ccm web`.
//
// The server listens on loopback only, holds every agent token from
// hosts.toml, and is the only thing the browser talks to. Design:
// docs/superpowers/specs/2026-10-01-web-ui-design.md.
package web

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"ccm/internal/api"
)

//go:embed static
var staticFiles embed.FS

// Options configures a Server.
type Options struct {
	ConfigPath string        // hosts.toml; a missing file means no hosts yet
	Secret     string        // from NewSecret; carried once by the opened URL
	Port       int           // port the server listens on, for the Host check
	PollEvery  time.Duration // agent polling interval while a browser is connected; 0 means 2s
}

type Server struct {
	hosts  *hostsFile
	secret string
	cookie string // session cookie value, distinct from the secret
	port   int
}

// New loads hosts.toml and prepares the server. A missing file is fine (the
// UI shows the first-run screen); a file that doesn't parse is an error.
func New(o Options) (*Server, error) {
	hosts, err := loadHostsFile(o.ConfigPath)
	if err != nil {
		return nil, err
	}
	return &Server{hosts: hosts, secret: o.Secret, cookie: randHex(32), port: o.Port}, nil
}

// Handler returns every route behind the Host check.
func (s *Server) Handler() http.Handler {
	return s.checkHost(s.routes())
}

func (s *Server) routes() *http.ServeMux {
	static, _ := fs.Sub(staticFiles, "static")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.Handle("GET /static/", s.page(http.StripPrefix("/static/", http.FileServerFS(static))))
	return mux
}

func (s *Server) serveIndex(w http.ResponseWriter, _ *http.Request) {
	b, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, api.Error{Error: err.Error()})
}
