package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"ccm/internal/hub"
)

// Close codes sent to the browser when the bridge can't reach the session.
const (
	closeNotFound    = 4404 // the agent has no such session
	closeUnreachable = 4502 // the agent could not be reached
)

// attach bridges the browser to the agent's attach WebSocket. The browser
// side is upgraded first, so a failure to reach the agent arrives as a close
// code and reason the page can show (a failed handshake shows nothing).
func (s *Server) attach(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("host")
	h, ok := s.hosts.Find(name)
	if !ok {
		writeErr(w, http.StatusNotFound, msgUnknownHost(name))
		return
	}
	browser, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade wrote the response, e.g. 403 for a foreign Origin
	}
	defer browser.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	agentConn, err := hub.NewClient(h).Dial(ctx, r.PathValue("id"))
	cancel()
	if err != nil {
		code := closeUnreachable
		var he *hub.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusNotFound {
			code = closeNotFound
		}
		reason := err.Error()
		if errors.As(err, &he) && (he.Code == http.StatusUnauthorized || he.Code == http.StatusForbidden) {
			reason = hub.ErrMsgAuth + ". Check hosts.toml."
		}
		closeWith(browser, code, reason)
		return
	}
	defer agentConn.Close()
	bridge(browser, agentConn)
}

// bridge copies frames both ways until either side closes. Each direction
// is the only writer of its destination connection.
func bridge(browser, agentConn *websocket.Conn) {
	done := make(chan struct{}, 2)
	go func() { pipe(agentConn, browser); done <- struct{}{} }()
	go func() { pipe(browser, agentConn); done <- struct{}{} }()
	<-done // the caller's deferred Close calls unblock the other direction
}

// pipe copies frames from src to dst until src fails. A close frame from src
// is passed on, so the agent's "session exited" reaches the browser.
func pipe(dst, src *websocket.Conn) {
	for {
		mt, data, err := src.ReadMessage()
		if err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				closeWith(dst, ce.Code, ce.Text)
			}
			return
		}
		if err := dst.WriteMessage(mt, data); err != nil {
			return
		}
	}
}

// closeWith sends a close frame. WriteControl is safe alongside the writer.
func closeWith(c *websocket.Conn, code int, reason string) {
	if len(reason) > 120 { // close frame payloads are limited to 125 bytes
		reason = reason[:120]
	}
	_ = c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
}
