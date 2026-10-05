package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/muesli/cancelreader"
	"golang.org/x/term"

	"ccm/internal/api"
)

// DetachKey is Ctrl-] (0x1d), same as telnet.
const DetachKey = 0x1d

// Attach takes over the local terminal and wires it to a remote session until
// the user presses DetachKey, the session exits, or the connection drops.
func Attach(ctx context.Context, c *Client, id string) error {
	inFd, outFd := int(os.Stdin.Fd()), int(os.Stdout.Fd())
	if !term.IsTerminal(inFd) || !term.IsTerminal(outFd) {
		return errors.New("attach needs an interactive terminal")
	}

	conn, err := c.Dial(ctx, id)
	if err != nil {
		return err
	}
	defer conn.Close()

	restoreConsole := prepareConsole()
	defer restoreConsole()
	oldState, err := term.MakeRaw(inFd)
	if err != nil {
		return err
	}
	defer term.Restore(inFd, oldState)

	var wmu sync.Mutex // gorilla/websocket: one writer at a time
	send := func(mt int, b []byte) error {
		wmu.Lock()
		defer wmu.Unlock()
		return conn.WriteMessage(mt, b)
	}
	sendSize := func(cols, rows int) {
		msg, _ := json.Marshal(api.Control{Type: api.ControlResize, Cols: cols, Rows: rows})
		_ = send(websocket.TextMessage, msg)
	}

	cols, rows, _ := term.GetSize(outFd)
	sendSize(cols, rows)

	stop := make(chan struct{})
	defer close(stop)

	// Poll for resizes: portable (Windows has no SIGWINCH) and cheap.
	go func() {
		t := time.NewTicker(250 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				if cc, rr, err := term.GetSize(outFd); err == nil && (cc != cols || rr != rows) {
					cols, rows = cc, rr
					sendSize(cols, rows)
				}
			}
		}
	}()

	// Cancellable so no reader is left blocked on stdin once Attach returns;
	// a caller that keeps running (the TUI) gets the terminal back intact.
	in, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return err
	}
	defer in.Close()

	detached := make(chan struct{})
	inputDone := make(chan struct{})
	defer func() {
		if in.Cancel() {
			<-inputDone
		}
	}()
	go func() {
		defer close(inputDone)
		if pumpInput(in, func(b []byte) error { return send(websocket.BinaryMessage, b) }) {
			close(detached)
		}
	}()

	type result struct {
		exitCode *int
		err      error
	}
	done := make(chan result, 1)
	go func() {
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				done <- result{err: err}
				return
			}
			switch mt {
			case websocket.BinaryMessage:
				_, _ = os.Stdout.Write(data)
			case websocket.TextMessage:
				var ctl api.Control
				if json.Unmarshal(data, &ctl) == nil && ctl.Type == api.ControlExit {
					done <- result{exitCode: ctl.Code}
					return
				}
			}
		}
	}()

	var res result
	select {
	case <-detached:
		wmu.Lock()
		_ = conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "detach"))
		wmu.Unlock()
		term.Restore(inFd, oldState)
		fmt.Printf("\r\n[detached from %s/%s]\r\n", c.Host.Name, id)
		return nil
	case res = <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	term.Restore(inFd, oldState)
	if res.exitCode != nil {
		fmt.Printf("\r\n[session %s/%s exited with code %d]\r\n", c.Host.Name, id, *res.exitCode)
		return nil
	}
	if websocket.IsCloseError(res.err, websocket.CloseNormalClosure) {
		return nil
	}
	return fmt.Errorf("connection lost: %w", res.err)
}

// pumpInput forwards r to send until r fails (EOF, cancel) or send fails, and
// reports whether it stopped because the user pressed DetachKey. Input before
// DetachKey in the same read is still sent; anything after it is dropped.
func pumpInput(r io.Reader, send func([]byte) error) (detached bool) {
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if i := bytes.IndexByte(chunk, DetachKey); i >= 0 {
				if i > 0 {
					_ = send(chunk[:i])
				}
				return true
			}
			if send(append([]byte(nil), chunk...)) != nil {
				return false
			}
		}
		if err != nil {
			return false
		}
	}
}
