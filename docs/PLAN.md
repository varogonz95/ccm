# Plan

Goal: one terminal on any PC shows every Claude Code session on every LAN machine with live status, and can attach, type, detach, start, and kill.

## Decisions

- One Go binary, two modes (`agent`, hub commands). CGO-free, cross-compiles for Windows, Linux, macOS.
- The agent owns the PTYs: creack/pty on Unix, ConPTY on Windows. No tmux dependency.
- Status via Claude Code hooks, not screen-scraping (M3).
- Attach is full-screen passthrough; Ctrl-] detaches.
- Shared bearer token per agent.

## Milestones

- [x] **M0, PTY spike.** Spawn in a PTY, I/O, resize, exit codes. Unix verified by tests; Windows compiles, needs a manual run.
- [x] **M1, agent API.** REST create/list/kill, WebSocket attach, scrollback replay, token auth, repaint nudge on attach.
- [~] **M2, hub.** Done: `hosts.toml`, `hosts`, `ls` across machines, `new`, `attach`, `kill`. Todo: bubbletea TUI (session list + attach + return to list), which needs a cancellable stdin reader.
- [ ] **M3, status and alerts.** Agent launches claude with `--settings` injecting `Stop`, `Notification`, `UserPromptSubmit` hooks that run `ccm hook <event>` (reads `CCM_SESSION_ID`, posts to the agent). New states: working, idle, needs_input. Hub shows badges and rings the bell on needs_input.
- [ ] **M4, hardening.** Agent as a service (systemd, launchd, Windows Service); hub auto-reconnect; Windows Job Object; multiple-viewer resize policy.

## Later

Split-pane multi-view; mDNS discovery; web frontend on the same API; TLS or Tailscale for off-LAN; session persistence across agent restarts.

## Manual Windows check (M0 exit criteria)

1. `ccm agent` on Windows, `ccm new <win-host>` from another machine.
2. Claude's TUI renders, arrow keys and Esc work, window resize reflows.
3. Detach, re-attach: screen repaints cleanly.
4. `ccm kill`: claude process gone in Task Manager.
