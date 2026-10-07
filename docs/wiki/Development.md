# Development

## Commands

```sh
make build        # ./clawsh
make test         # go test -race ./...
make vet          # go vet for linux, windows and darwin
make dist         # cross-compile into dist/
make docker-dist  # same, inside Docker
```

Run one test:

```sh
go test -race -run TestReplayOnLateAttach ./internal/agent
```

The end-to-end tests (`internal/agent/e2e_test.go`) start a real agent over HTTP/WebSocket with `/bin/sh` standing in for claude. They're skipped on Windows, which needs a manual check (see `docs/PLAN.md`).

For a manual loop, run `./clawsh agent --claude /bin/sh` and point a `hosts.toml` at `http://localhost:7420`.

## Releases

`clawsh version` prints the version baked in at build time. The Makefile takes it from `git describe` (`v0.1.0` → `0.1.0`, untagged builds get the commit); override with `make dist VERSION=1.2.3`. A plain `go build` reports `dev`.

To publish, bump `plugin/.claude-plugin/plugin.json` to match, then push a tag:

```sh
git tag v0.1.0 && git push origin v0.1.0
```

`.github/workflows/release.yml` runs the CI checks (`make vet`, `make test`), cross-compiles with `make dist`, packages one archive per platform plus `checksums.txt`, and creates the GitHub release with generated notes. Tags with a suffix (`v0.1.0-rc.1`) become pre-releases. `.github/workflows/ci.yml` runs the same checks on every push to `main` and on pull requests.

## Layout

| Path | |
|---|---|
| `cmd/clawsh` | CLI entrypoint, every subcommand |
| `internal/api` | Wire types shared by agent and hub |
| `internal/ptyx` | PTY abstraction: creack/pty on Unix, ConPTY on Windows |
| `internal/agent` | Session manager, HTTP/WebSocket server, token, scrollback |
| `internal/hub` | `hosts.toml`, agent client, terminal attach |

## Rules

- Everything builds for Windows, Linux and macOS with `CGO_ENABLED=0`. OS-specific code goes in `_windows.go` / `_unix.go` / `_other.go` files with build tags. Run `make vet` after touching them.
- The agent never takes the executable from the API, only args.
- gorilla/websocket allows one writer per connection. Keep the single-writer pattern.
- A slow viewer is dropped, never allowed to block a session.
- Keep dependencies minimal; prefer the standard library.
- New endpoints or control messages: add types to `internal/api` first, then a test in `e2e_test.go`.
