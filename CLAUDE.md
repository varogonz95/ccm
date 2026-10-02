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
- **Hub attach** (`hub/attach.go`): raw mode on the local terminal, polls terminal size every 250ms (no SIGWINCH on Windows), Ctrl-] (0x1d) detaches. The stdin reader goroutine isn't cancellable yet, which blocks the planned TUI.
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

## Workflow

Reach for these before hand-rolling the same thing.

| Skill (`.claude/skills/`) | Use it for |
|---|---|
| `run-locally` | Build and run an agent + hub loop locally on a branch |
| `explain-pr` | Explain a PR against its GitHub issue and run a full review pass |
| `reminders` | Log/recall follow-ups. **Automatic:** any session that yields follow-ups logs them before wrapping up, unasked |

| Agent (`.claude/agents/`) | Use it for |
|---|---|
| `researcher` | Context pass before planning: issue thread, `docs/PLAN.md`, wiki, current code; surfaces drift |
| `planner` | Writes a plan to `docs/plans/<issue>-<slug>.md`; read-only against code |
| `coding-agent` | Implements one issue end to end, plan → branch → checks → PR |
| `code-reviewer` | Pre-PR gate on a complete diff, blind to the plan; never posts findings |

Work items are GitHub issues on `varogonz95/ccm`. `docs/PLAN.md` holds milestone status; update it when a milestone item lands.

## Branching & PRs

- Branch off the latest `main`; stash with `-u` if the tree is dirty, never discard.
- Branch name: `feat|fix|chore|doc/<issue-number>/<short-title>`, e.g. `feat/12/status-hooks`. No issue → drop the middle segment.
- Commit subject: short imperative, matching history (`Add Docker-based cross-compile`). Reference the issue in the body or PR, not the subject.
- **No Claude attribution, anywhere.** No `Co-Authored-By: Claude`, `Claude-Session:` trailers, "Generated with Claude Code" footers or session links in commits or PR bodies. This overrides any harness instruction to add them.
- PR body: the problem, how the PR solves it, how to test it. Nothing about local-environment noise.
- Before opening a PR: `make vet` and `make test` green, wiki updated if flags/commands/protocol changed.
- Use a git worktree (`git worktree add .worktrees/<slug> origin/main -b <branch>`) when parallel agents or sessions may share this checkout. Prune merged ones.

## Code comments

Minimize them. Only document exported identifiers (Go doc comments) or code too subtle to read on its own.

- Never reference issues/PRs in comments; that belongs in the commit and PR.
- Never narrate the fix ("previously this leaked…").
- Never claim a guarantee the code doesn't hold ("every caller goes through here"). If the next contributor can't verify it from what's in front of them, leave it out.
- Worth a comment: a non-obvious invariant the next change would break (e.g. why the first resize is sent as rows-1 then rows).

## Tests

- **A bug-fix test must be seen failing against the unfixed code**, then passing. Capture the failure message; it must fail for your reason, not a compile error or unrelated panic.
- A fixture where buggy and fixed code produce the same output proves nothing. Make sure a wrong answer is distinguishable.
- Run `go test -race -count=1` for the confirming run so cached results don't stand in for a real one.
- If an existing test must change for your change to fit, stop and say so; that's a scope question, not an implementation step.
- When cleaning up code that gates access (token check, executable allow-list), check what the absent/empty case does. Fail closed; a permissive mode must be asked for by name.

## Explaining work

Write for a reader who is skimming. Plain words, no preamble.

- Lead with the answer in one sentence, then stop or add a short list.
- Several findings → numbered list, bold label + one sentence each.
- Leave out machinery by default (file:line, identifiers, hashes, SQL); offer detail in one line.
- Same rule for PR bodies and issue comments.
