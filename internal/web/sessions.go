package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"ccm/internal/api"
	"ccm/internal/hub"
)

func msgUnknownHost(name string) error { return fmt.Errorf("unknown machine %q", name) }

// client returns an agent client for the {host} path value, or writes 404.
func (s *Server) client(w http.ResponseWriter, r *http.Request) (*hub.Client, bool) {
	name := r.PathValue("host")
	h, ok := s.hosts.Find(name)
	if !ok {
		writeErr(w, http.StatusNotFound, msgUnknownHost(name))
		return nil, false
	}
	return hub.NewClient(h), true
}

func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	c, ok := s.client(w, r)
	if !ok {
		return
	}
	var req api.CreateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	sess, err := c.Create(r.Context(), req)
	if err != nil {
		relayErr(w, err)
		return
	}
	s.bc.kick()
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	c, ok := s.client(w, r)
	if !ok {
		return
	}
	if err := c.Kill(r.Context(), r.PathValue("id")); err != nil {
		relayErr(w, err)
		return
	}
	s.bc.kick()
	w.WriteHeader(http.StatusNoContent)
}

// relayErr passes an agent's status and message through; anything else
// (unreachable agent, timeout) becomes 502.
func relayErr(w http.ResponseWriter, err error) {
	var he *hub.HTTPError
	if errors.As(err, &he) {
		msg := he.Msg
		if msg == "" {
			msg = he.Status
		}
		writeErr(w, he.Code, errors.New(msg))
		return
	}
	writeErr(w, http.StatusBadGateway, err)
}
