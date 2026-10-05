# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

# ccm

Cross-machine manager for Claude Code sessions. Go 1.22, single binary with two roles: `ccm agent` (runs on each machine, owns `claude` processes in PTYs, serves REST + WebSocket) and the hub commands (`hosts`, `ls`, `new`, `attach`, `kill`) run from any terminal. Plan and milestone status: `docs/PLAN.md` (M0/M1 done, M2 partly done, M3 status via hooks next).

## Layout

- `cmd/ccm` — CLI entrypoint; all subcommands (agent + hub side) in one `main.go`.
- `internal/api` — wire types shared by agent and hub. Protocol is documented at the top of `types.go`.
- `internal/ptyx` — PTY abstraction; `_unix.go` (creack/pty) and `_windows.go` (ConPTY).
- `internal/agent` — session manager, HTTP/WebSocket server, token, scrollback.
- `internal/hub` — hosts config, agent client, terminal attach.
- `internal/web` — `ccm web`: loopback server with access-key auth (header or `?k=`), SSE overview, attach bridge, and the embedded UI in `static/` (plain JS, vendored xterm.js; see `static/vendor/VERSIONS`).
- `internal/mcp` — `ccm mcp`: tool-less MCP stdio stub run by the Claude Code plugin; spawns a detached agent and announces the session (`/v1/external` lease). PoC, see `docs/poc-auto-agent.md`.
- `plugin/` — the Claude Code plugin (`.mcp.json` runs `ccm mcp`); `.claude-plugin/marketplace.json` at the root makes the repo a marketplace.
- `docs/wiki` — user docs (GitHub-wiki style pages, `Home.md` is the index). Update them when flags, commands or the protocol change.
- `site/index.html` — static landing page, deployed to GitHub Pages by `.github/workflows/pages.yml`.

## Commands

- `make build` — builds `./ccm`.
- `make test` — race-enabled tests; `internal/agent/e2e_test.go` drives a real server with `/bin/sh` as a stand-in for claude.
- Single test: `go test -race -run TestReplayOnLateAttach ./internal/agent`
- `make vet` — vets for linux, windows, darwin. Run it after any change touching `ptyx`, `hub/console_*`, or syscalls.
- `make dist` — cross-compile all platforms into `dist/`.
- `make docker-dist` — same cross-compile inside Docker (`Dockerfile`, buildx), exported to `dist/`; no local Go needed.
- Local manual run: `./ccm agent --claude /bin/sh` (any command works as the session program), then point a `hosts.toml` (path from `ccm help`; example in `hosts.example.toml`) at `http://localhost:7420` with the token from `./ccm token`.

## Architecture

- **Session lifecycle** (`agent/session.go`): each `Session` runs two goroutines. `readLoop` reads PTY output, appends to a bounded `Scrollback`, and fans out to subscriber channels (non-blocking send; a full channel means the viewer is dropped). `waitLoop` records the exit code, waits briefly for the reader to drain, then closes the PTY (on Windows, ConPTY reads only unblock on close) and closes `done`.
- **Attach** (`agent/server.go`): `Session.Attach()` returns a scrollback snapshot plus a live subscription under one lock, so no output is lost or duplicated between replay and live stream. The handler goroutine is the sole WebSocket writer (snapshot, then chunks, pings, and a final `exit` control). `readClient` forwards binary frames to the PTY and handles `resize`; the first resize is deliberately sent as rows-1 then rows to force claude to repaint after replay.
- **Hub attach** (`hub/attach.go`): raw mode on the local terminal, polls terminal size every 250ms (no SIGWINCH on Windows), Ctrl-] (0x1d) detaches. Stdin is read through `cancelReader` (`hub/stdin_*.go`: select + self-pipe on unix, console wait + peek on Windows, never blocking in a read without input). Every websocket write has a deadline (`writeTimeout`), so a dead connection ends the attach. On return a single teardown cancels the reader, closes the connection, waits for all attach goroutines, then restores the terminal before any status line is printed, so a long-lived caller (the planned TUI) gets a clean terminal and stdin back.
- **Web UI** (`web/`): the browser only talks to `ccm web`, never to agents. `events.go` polls `hub.Overview` while any tab is subscribed and pushes only changes; `attach.go` bridges WebSockets with one writer per connection. Close code 4404 tells the page the session is gone.
- **Targets**: hub commands address sessions as `host/idprefix`; host names come from `hosts.toml`.
- Sessions get `CCM_SESSION_ID` in their env (groundwork for M3 hook callbacks). Sessions do not survive an agent restart.

## Rules

- Everything must build for windows, linux and darwin with `CGO_ENABLED=0`. OS-specific code goes in `_windows.go` / `_unix.go` / `_other.go` files with build tags.
- The agent never takes the executable from the API; only args (executable is fixed by `ccm agent --claude`). Keep it that way.
- gorilla/websocket allows one concurrent writer per connection. Keep the single-writer pattern (agent: the attach loop; hub: `send` with mutex).
- A slow viewer is dropped, never allowed to block a session's read loop.
- Keep dependencies minimal; prefer the stdlib.
- New endpoints or control messages: add types to `internal/api` first, then a test in `e2e_test.go`.
- `e2e_test.go` is `!windows`-tagged; Windows behavior needs the manual check listed in `docs/PLAN.md`.
