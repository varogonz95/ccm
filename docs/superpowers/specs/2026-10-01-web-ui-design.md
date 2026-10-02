# Web UI for the ccm hub (`ccm web`)

Issue: #5 (parent #4). Status: design approved in brainstorming, awaiting spec review.

## Goal

People who avoid terminals can run one command, get a browser tab, see every machine and Claude Code session, open a session in a real terminal, start new ones and end them. The same UI is the base for a later desktop client.

Success: from a fresh `hosts.toml`, a user runs `ccm web`, the browser opens, and without typing any other command they can start a session on any online machine, work in it, leave, come back to it, and end it.

## Scope

In scope (v1):

- `ccm web` command: local server on loopback, embedded UI, opens the browser.
- Dashboard of all hosts in `hosts.toml` with live updates (SSE).
- Full-page attached terminal (xterm.js).
- New session dialog, end session, dismiss exited sessions.
- First-run screen when no hosts are configured.
- Screenshots of the three main views for the landing page work in #9.

Out of scope (own issues):

- Editing `hosts.toml` from the UI: #8.
- Opening the UI from other LAN devices, TLS, mobile terminal input: #6.
- UI served by each agent: #7.
- Landing page showcase with tilted snapshots: #9.
- Desktop client: later, wraps this UI.
- M3 status badges (working, needs input): added to the overview payload when M3 lands.

## Decisions

| Topic | Decision | Why |
|---|---|---|
| Where it runs | Local server on the hub machine, `127.0.0.1` only | Tokens stay in `hosts.toml`; avoids browser limits (no bearer header on WebSocket, no CORS on agents, mixed content) |
| Frontend | Plain HTML/CSS/JS, vendored xterm.js + fit addon, `go:embed` | Go-only build, no Node toolchain, minimal deps; about four views |
| Local auth | Per-launch random secret in the opened URL, exchanged for a cookie; Host and Origin checks | Blocks other local users and hostile websites (CSRF, cross-site WebSocket, DNS rebinding) |
| Backend shape | Aggregated overview + per-host endpoints through `hub.Client`; browser never talks to agents | One request for the dashboard, explicit allowlist of operations |
| Live updates | Server-sent events, server polls agents | Ready for M3 events without a protocol change on the browser side |
| Layout | Dashboard of host cards, then a full-page terminal (option B) | Matches the intended mental model; terminal gets the whole page |

Design canvas (approved mockups): https://claude.ai/artifact/Uec5srZ66rCvfLaUNP1XDg

## Architecture

```
browser ──cookie──▶ ccm web (127.0.0.1:7421) ──bearer──▶ agent A, B, C…
   SSE /api/events    ◀── polls hub.Overview every 2s while a tab is connected
   WS  …/attach       ◀── bridges to the agent's attach WebSocket
```

### Command

`ccm web [--listen 127.0.0.1:7421] [--no-open] [--config path]`

- Loads `hosts.toml` (default path as other hub commands). A missing file or one with no hosts is not an error: the UI shows the first-run screen. A file that fails to parse is a startup error.
- While running, the poller re-reads `hosts.toml` when its modification time changes. A later parse error keeps the last good config and logs the error.
- `--listen` must resolve to a loopback address; anything else is refused with a pointer to #6.
- Generates a 32-byte random secret (hex), prints `ccm web: open http://127.0.0.1:7421/?k=<secret>` and, unless `--no-open`, opens that URL in the default browser.
- Runs until interrupted.

### Packages and files

`internal/web` (new):

| File | Responsibility |
|---|---|
| `server.go` | `New(cfg, secret) *Server`, route table, create/delete handlers wrapping `hub.Client` |
| `auth.go` | Secret-to-cookie exchange, cookie check middleware, Host check, WebSocket Origin check |
| `events.go` | SSE endpoint and broadcaster: one poller goroutine shared by all subscribers, started on first subscriber, stopped on last; pushes only when the overview changed; drops a subscriber whose buffer is full |
| `attach.go` | Bridges browser WebSocket ↔ agent WebSocket; frames passed through unchanged; one writer goroutine per connection on each side |
| `open_unix.go`, `open_windows.go` | Open a URL in the default browser (`xdg-open` / `open` on darwin via `runtime.GOOS`; `rundll32 url.dll,FileProtocolHandler` on Windows) |
| `static/` | `index.html`, `app.js`, `app.css`, `vendor/` (xterm.js, xterm.css, addon-fit.js, `VERSIONS` with pinned versions and source URLs) |

`internal/hub/overview.go` (new): `Overview(ctx, hosts []Host) []api.HostOverview`. Queries health and session list for every host in parallel with a per-host timeout (3s). Replaces the fan-out currently inside `runHosts` and `runList` in `cmd/ccm/main.go`; both commands switch to it.

`internal/api` gains:

```go
// HostOverview is one host as seen by the hub: health plus sessions, or why it is unreachable.
type HostOverview struct {
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Online   bool      `json:"online"`
	Health   *Health   `json:"health,omitempty"`
	Error    string    `json:"error,omitempty"`
	Sessions []Session `json:"sessions"`
}
```

`HostOverview.URL` is shown on the card (address only); tokens are never part of any response.

No agent changes. Agent `DELETE /v1/sessions/{id}` already stops a running session and forgets an exited one.

## HTTP API (ccm web)

Every route except the secret exchange requires the session cookie. Every route checks `Host` is `127.0.0.1:<port>` or `localhost:<port>`.

| Route | Behaviour |
|---|---|
| `GET /?k=<secret>` | Constant-time compare; on match set cookie `ccm_web` (random value distinct from the secret, HttpOnly, SameSite=Strict, Path=/) and 303 to `/`. On mismatch 401 page. |
| `GET /`, `GET /static/*` | Embedded UI. Without cookie: 401 page "Open the link printed by `ccm web`." |
| `GET /api/events` | `text/event-stream`. Event `overview` with `[]api.HostOverview` on connect and on every change. Comment ping every 20s. |
| `POST /api/hosts/{host}/sessions` | Body `api.CreateRequest` → 201 `api.Session`. Unknown host 404. Agent errors relayed as `api.Error` with the agent's status. |
| `DELETE /api/hosts/{host}/sessions/{id}` | → 204. Relays agent errors. |
| `GET /api/hosts/{host}/sessions/{id}/attach` | WebSocket. Requires same-origin `Origin`. Dials agent via `hub.Client.Dial`; on dial failure closes with a close frame carrying the error text. |

Cookie value and secret live only in memory; restarting `ccm web` invalidates both.

## UI

Four views, matching the design canvas. Wording targets non-terminal users: "machine" for host, "access key" for token, "End session" for kill. CLI and API names are unchanged.

1. **Dashboard** (`#/`)
   - Header: `ccm` wordmark, "Live" indicator (EventSource state), "New session" button.
   - "Your machines" heading with summary ("2 of 3 online · 3 sessions running").
   - One card per host, in `hosts.toml` order: initial-letter tile in a per-host colour (stable, derived from host order), name, OS and address, Online/Offline pill.
   - Running session rows link to the session view; they show name, folder, viewer count when > 0, age.
   - Exited sessions show "Exited · code N" and a Dismiss button (`DELETE`).
   - "New session on <host>" link per online card, preselecting that host.
   - Offline card: dashed, shows the error text and "Check that it's on and running `ccm agent`". A 401 from the agent shows "Access key rejected. Check hosts.toml."
2. **Session** (`#/s/<host>/<id>`)
   - "All machines" back link, host tile, session name, End session button.
   - Chips: folder, "Started N ago", viewers ("Only you are viewing" when 1), connection state.
   - xterm.js terminal filling the remaining height; fit addon sends `resize` on open and on window resize.
   - Under the terminal: "Leaving this page only disconnects you. Claude keeps working on <host>, and you can come back any time."
   - End session asks for confirmation ("End <name> on <host>? Claude will stop."), then `DELETE` and returns to the dashboard.
   - On `exit` control: banner "Claude exited (code N)" with "Back to machines"; terminal stays readable.
   - On WebSocket drop while the session still exists in the overview: "Disconnected, reconnecting…", retry with backoff (1s doubling to 10s); replay restores the screen.
3. **New session** (dialog over the dashboard)
   - Machine radio cards (offline disabled), Folder (empty = agent's home), Name (optional), Advanced → Extra Claude arguments (split on whitespace).
   - Sends current terminal-area size as `Cols`/`Rows`. On success navigates to the new session. Agent errors shown inline.
4. **First run** (no hosts configured)
   - Three steps: run `ccm agent`, copy the key with `ccm token`, add a `[[host]]` block to the hosts file (actual path from the server).
   - No "Check again" button (the canvas shows one): the poller re-reads `hosts.toml` when its modification time changes, so the page updates by itself once the file is saved. Copy: "This page updates by itself when you save the file."
   - "Adding machines from this page is coming later." (#8)

Visual language from the canvas: dark ground, Bricolage Grotesque for UI text and JetBrains Mono for paths and terminal, landing-page host colours, 44px minimum touch targets. Fonts are vendored as woff2 under `static/vendor/fonts/` rather than loaded from Google Fonts, so the UI works offline. Dark theme only in v1.

## Error handling

- Per-host timeout inside `hub.Overview`; a failed host becomes `Online:false` with `Error`. One slow host never delays others.
- SSE subscriber buffer of 4 events; a full buffer drops the subscriber (browser reconnects automatically). The poller never blocks on a subscriber.
- The attach bridge closes both sides when either side closes or errors; the agent's `exit` control is forwarded before close.
- Create/delete errors are surfaced in the UI, never swallowed.
- Bad cookie / Host / Origin: 401/403 with a short HTML explanation for page routes, `api.Error` JSON for `/api/*`.

## Testing

Go tests in `internal/web`, `!windows`-tagged like `e2e_test.go`, using `httptest` and in-process agents (`agent.NewServer` with `/bin/sh`):

- Auth: secret exchange sets cookie; missing/wrong cookie → 401; foreign `Host` → 403; attach with foreign `Origin` → 403; secret is single-purpose (cookie value ≠ secret).
- Overview: one live agent + one closed port → first SSE event has one online and one offline host; creating a session produces a new event containing it; no event when nothing changed.
- Bridge: create through `POST`, attach, echo bytes round-trip, second attach gets replay, `DELETE` → client receives `exit` control.
- Config reload: adding a host to `hosts.toml` while a subscriber is connected produces an overview event containing it; writing an invalid file keeps the previous hosts.

`internal/hub`: unit test for `Overview` (timeout, error mapping, order preserved). Existing CLI behaviour of `ccm ls` and `ccm hosts` unchanged after the refactor.

Manual checklist (added to `docs/PLAN.md`): dashboard updates live when a session starts from the CLI; attach renders Claude correctly, resize reflows; leave and return repaints; End session; exit banner; offline host card; first-run screen; Windows: `ccm web` opens the browser.

`make vet` for all three OSes (new `open_*.go` files).

## Docs and deliverables

- `docs/wiki/Web-UI.md` (new), linked from `Home.md` and `_Sidebar.md`.
- `docs/wiki/Commands.md`: `ccm web` and flags.
- `docs/wiki/Security.md`: localhost model, secret/cookie, what it does not protect.
- `docs/wiki/Roadmap.md` and `docs/PLAN.md`: web UI milestone, links to #6, #7, #8, #9.
- Screenshots of dashboard, session and new-session views at 1280px wide, saved to `site/img/` for #9.
