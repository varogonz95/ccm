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

## The web UI (`ccm web`)

`ccm web` holds every token in `hosts.toml`, so it guards itself:

- It listens on loopback only and refuses other `--listen` addresses.
- Each launch makes a random access key. The printed link carries it to the page, which keeps it in the browser's storage for that exact address (port included) and removes it from the address bar. Every API request must carry it. Restarting `ccm web` invalidates the old key.
- The key is deliberately not a cookie: browsers send a host's cookies to every port on it, so another user's server on another local port could collect one.
- Requests whose `Host` isn't `127.0.0.1`/`localhost` on its port are refused (DNS rebinding). WebSockets and write requests from other origins are refused (cross-site requests).
- The browser never receives tokens or talks to agents directly.

It does not protect against software already running as your user on that computer, which could read `hosts.toml` anyway.
