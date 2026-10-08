# Roadmap and limits

The full plan is in [`docs/PLAN.md`](../PLAN.md).

## Milestones

- **M0, PTY layer:** done. Unix verified by tests; Windows compiles and needs a manual run.
- **M1, agent API:** done. Create, list, kill, attach, scrollback replay, token auth.
- **M2, hub:** mostly done. `hosts`, `ls`, `new`, `attach`, `kill` work. Still to come: an interactive TUI with a session list you can attach to and return from.
- **Web UI:** done. `clawsh web` serves a browser dashboard with a full-page terminal (loopback only). Next: LAN access (#6), editing hosts from the page (#8).
- **M3, status and alerts:** next. Claude Code hooks will report `working`, `idle` and `needs_input`; the hub will show badges and ring the bell when a session needs you.
- **Session transfer:** planned after M3. `clawsh move` will stop an idle session, copy its conversation and your uncommitted work to another machine, resume it there, and keep an archived record on the old machine. Works for sessions started outside clawsh too. Later, sessions could move on their own to the machine with the most free resources.
- **M4, hardening:** agent as a system service, hub auto-reconnect, Windows Job Objects, a resize policy for multiple viewers.

Later: split-pane views, mDNS discovery, TLS or Tailscale, sessions that survive agent restarts.

## Known limits

- Only sessions started through clawsh are managed, not `claude` launched in a plain terminal.
- Sessions stop when the agent stops. `claude --resume` recovers the conversation.
- Status is `running` or `exited` only, until M3.
- On Windows, killing a session doesn't yet kill claude's child processes.
- With several viewers, the last resize wins.
- On Windows, typing into a session right after it exits can misbehave in the agent. Tracked in issue #12.
