# Security

**Anyone holding an agent's token can run Claude Code on that machine, with that machine user's permissions.** Treat tokens like SSH keys.

## What protects you today

- Every endpoint except `/v1/health` needs the bearer token, compared in constant time.
- The executable is fixed on the agent (`--claude`). The API accepts extra arguments only, so a client can't run an arbitrary program. Arguments still reach claude, so a token holder controls claude's flags.
- Token files are written with mode `0600` in a `0700` directory.

## What doesn't

- **Traffic is plain HTTP.** Tokens and terminal contents cross the network unencrypted. Keep agents on a trusted LAN until TLS or Tailscale support lands (see [Roadmap](Roadmap.md)).
- `/v1/health` is open and reveals hostname, OS and version.
- The agent listens on all interfaces by default. Use `--listen 192.168.1.20:7420` (or a VPN address) to narrow it.

## Rotating a token

Stop the agent, delete its `agent.token` (see [Configuration](Configuration.md)), start it again, and update `hosts.toml` wherever it's used.
