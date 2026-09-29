# ccm

Cross-machine manager for Claude Code sessions. Go, single binary. Plan and milestone status: `docs/PLAN.md`.

## Layout

- `cmd/ccm` — CLI entrypoint; all subcommands (agent + hub side).
- `internal/api` — wire types shared by agent and hub. Protocol is documented at the top of `types.go`.
- `internal/ptyx` — PTY abstraction; `_unix.go` (creack/pty) and `_windows.go` (ConPTY).
- `internal/agent` — session manager, HTTP/WebSocket server, token, scrollback.
- `internal/hub` — hosts config, agent client, terminal attach.

## Commands

- `make test` — race-enabled tests; `internal/agent/e2e_test.go` drives a real server with `/bin/sh` as a stand-in for claude.
- `make vet` — vets for linux, windows, darwin. Run it after any change touching `ptyx`, `hub/console_*`, or syscalls.
- `make dist` — cross-compile.

## Rules

- Everything must build for windows, linux and darwin with `CGO_ENABLED=0`. OS-specific code goes in `_windows.go` / `_unix.go` / `_other.go` files with build tags.
- The agent never takes the executable from the API; only args. Keep it that way.
- gorilla/websocket allows one concurrent writer per connection. Keep the single-writer pattern (agent: the attach loop; hub: `send` with mutex).
- A slow viewer is dropped, never allowed to block a session's read loop.
- Keep dependencies minimal; prefer the stdlib.
- New endpoints or control messages: add types to `internal/api` first, then a test in `e2e_test.go`.
