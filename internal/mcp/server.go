// Package mcp is the `ccm mcp` stub: a minimal MCP stdio server that a
// Claude Code plugin registers so it runs for the lifetime of every Claude
// Code session. It exposes no tools; it exists for its side effects:
//
//   - make sure a local `ccm agent` is running, spawning a detached one if not;
//   - announce the session to that agent (a renewed lease) so `ccm ls` sees
//     claude sessions started outside ccm, and withdraw it on exit.
//
// Claude Code closes stdin when the session ends, which ends Serve.
package mcp

import (
	"bufio"
	"encoding/json"
	"io"
	"sync"

	"ccm/internal/api"
)

// MCP JSON-RPC 2.0 over newline-delimited stdio.
type message struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FallbackProtocolVersion is answered when the client doesn't send one.
const FallbackProtocolVersion = "2025-06-18"

// Serve answers MCP requests on r/w until r is closed. It advertises no
// capabilities, but answers the list methods with empty results in case a
// client asks anyway.
func Serve(r io.Reader, w io.Writer) error {
	var mu sync.Mutex
	enc := json.NewEncoder(w)
	send := func(m message) {
		mu.Lock()
		defer mu.Unlock()
		m.JSONRPC = "2.0"
		_ = enc.Encode(m) // Encode appends the newline delimiter
	}

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	for sc.Scan() {
		var req message
		if json.Unmarshal(sc.Bytes(), &req) != nil {
			send(message{ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		if len(req.ID) == 0 || req.Method == "" {
			continue // notification (e.g. notifications/initialized) or a response
		}
		res, rerr := handle(req)
		send(message{ID: req.ID, Result: res, Error: rerr})
	}
	return sc.Err()
}

func handle(req message) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		v := p.ProtocolVersion
		if v == "" {
			v = FallbackProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{},
			"serverInfo":      map[string]any{"name": "ccm", "version": api.Version},
			"instructions":    "ccm bookkeeping only: keeps a local ccm agent running and lists this session in `ccm ls`. No tools.",
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": []any{}}, nil
	case "resources/list":
		return map[string]any{"resources": []any{}}, nil
	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil
	default:
		return nil, &rpcError{-32601, "method not found: " + req.Method}
	}
}
