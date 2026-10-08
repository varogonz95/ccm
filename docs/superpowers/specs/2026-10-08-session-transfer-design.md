# Session transfer between machines (`clawsh move`)

Status: idea, design draft. Depends on M3 (hooks) for idle detection and the claude session id.

## Goal

Move a running Claude Code session from one machine to another and keep working there with the full conversation, as if it had started on the target.

Use cases:

- The source machine is short on CPU or memory; move heavy sessions off it.
- Move sessions to a laptop before travelling, so they keep running locally offline.
- Free a machine for a reboot or update without losing context.
- Later: automatic placement on the machine with the most headroom.

Success: `clawsh move pc1/a3f laptop` stops the session on `pc1`, and a minute later `clawsh attach laptop/<id>` shows the same conversation, in the same project, with claude able to keep editing files.

## What "session state" is

Claude Code already persists everything it needs after every message, so there is nothing to ask claude to dump:

| Data | Where (on the source) | Needed |
|---|---|---|
| Transcript | `~/.claude/projects/<encoded cwd>/<session-id>.jsonl` | yes |
| Checkpoints (rewind) | `~/.claude/file-history/<session-id>/` | nice to have |
| Todos | `~/.claude/todos/<session-id>-*.json` | nice to have |
| Workspace | the session's `Dir` | yes, see below |
| Project settings | `<Dir>/.claude/`, `CLAUDE.md` | comes with the workspace |

`<encoded cwd>` is the absolute path with separators and dots replaced by `-`. Each transcript line carries a `cwd` field.

Not transferable: background processes claude started (dev servers, watchers), MCP server state, the shell environment and secrets, tools installed only on the source. The target must have claude, the same MCP servers configured, and the toolchain the project needs.

## Flow

1. **Quiesce.** Wait until the session is idle (M3 `Stop` hook: claude finished its turn). Moving mid-turn would cut a tool call in half, so `move` refuses with "session is working" unless `--wait` (block until idle) or `--force`.
2. **Stop.** The source agent ends claude gracefully (`/exit`, then Ctrl-C, then kill), so the last transcript line is flushed.
3. **Export.** The source agent packs a tarball: transcript, file-history, todos, a manifest (session id, source `Dir`, source home, OS, claude version), and optionally the workspace (see below).
4. **Ship.** The hub streams `GET` from the source agent into `PUT` on the target agent. Both endpoints already use the bearer token in `hosts.toml`.
5. **Import and rewrite.** The target agent maps the source `Dir` to a target `Dir`, rewrites paths (below), writes files under its own `~/.claude`, and starts `claude --resume <session-id>` in the target `Dir` as a normal clawsh session.
6. **Confirm.** Only after the target session is running does the hub tell the source to delete its copy (or keep it, `--keep`). A failed import leaves the source untouched and the session can be restarted there with `--resume`.

### Why not scp

scp needs SSH between every pair of machines, keys on both sides, and OpenSSH on Windows. The agents already speak authenticated HTTP to the hub, so the hub relays the stream. A direct agent-to-agent path can come later for large workspaces.

## Workspace

The transcript is useless if the target has no copy of the code, and this is the hardest part. Three modes, picked per move:

- **`--workspace=present`** (default): the project already exists on the target, at a path from a mapping in `hosts.toml` (below), or given with `--dir`. Fits repos cloned on every machine and shared folders (Syncthing, network drives). Before stopping, clawsh checks the target path exists and, for git repos, warns if `HEAD` differs from the source.
- **`--workspace=git`**: the target has the repo but not the local changes. Ship the source `HEAD` commit id, unpushed commits as a `git bundle`, and uncommitted plus untracked (non-ignored) files as a tar. The target fetches the bundle, checks out a branch at that commit, and lays the files on top. Refuses if the target tree is dirty.
- **`--workspace=copy`**: tar the whole `Dir`, honouring `.gitignore` when it is a git repo. Simple, can be large; a size cap applies (`--max-size`).

Path mapping in `hosts.toml`:

```toml
[[host]]
name = "laptop"
url = "http://192.168.1.31:7420"
token = "paste-token-from-laptop"
# source prefix on any host -> prefix on this host
paths = { "/home/me/src" = 'C:\Users\me\src' }
```

## Path rewriting

Done on the target, on the transcript and todos:

- Rename the project folder to the encoding of the target `Dir`.
- Rewrite the `cwd` field on every line.
- Best effort: replace the source `Dir` prefix and the source home prefix with the target ones in tool inputs and results, converting separators when the OS differs. Historical tool output is text; a missed path only affects what claude reads back, not correctness of new tool calls, which use the new `cwd`.
- Leave the session id unchanged so `--resume <id>` works. On collision (same id already on the target), refuse.

The rewrite is a pure function with golden-file tests (Linux to Linux, Linux to Windows, Windows to macOS).

## Getting the claude session id

clawsh does not know it today. Options:

- M3 `SessionStart` hook: its input has `session_id` and `transcript_path`; `clawsh hook` posts both to the agent. Works for `--resume` and `--continue` sessions too. Preferred.
- Launch new sessions with `--session-id <uuid>` chosen by the agent. Simple, but misses sessions that switch transcripts.

## API (to add to `internal/api` first)

- `POST /v1/sessions/{id}/stop` `{wait: bool}`: graceful stop, keeps transcript, returns the claude session id. Distinct from `DELETE` (kill).
- `GET /v1/sessions/{id}/export?workspace=present|git|copy`: streams the tarball. Session must be stopped.
- `PUT /v1/import` (body: tarball, query: target `dir`, name, cols, rows): validates, rewrites, writes, resumes; returns the new `api.Session`.
- `DELETE /v1/exports/{claude-session-id}`: removes the source copy after a confirmed move.

Hub: `clawsh move <host/id> <target-host> [--dir path] [--workspace mode] [--wait|--force] [--keep]`. Web UI: a "Move to..." action on the session card.

## Safety

- The executable stays fixed by `clawsh agent --claude`; import only adds `--resume <id>`.
- Import validates every tar entry: no absolute paths, no `..`, no symlinks out of the destination, size cap.
- Import writes only under the agent user's `~/.claude` and under the target `Dir`; the target `Dir` must be inside an allowlist (`clawsh agent --workspace-root`, defaults to home). Without that, a token holder could write anywhere the agent user can.
- The workspace tarball can contain secrets (`.env`); `copy` mode warns and honours `.gitignore`. Transfer is plain HTTP on the LAN until TLS lands (see `docs/wiki/Security.md`).

## Phases

1. **T1:** `stop`, `export`, `import` with `--workspace=present` and path mapping; transcript rewrite with tests; `clawsh move`. Same-OS only.
2. **T2:** `git` and `copy` workspace modes.
3. **T3:** cross-OS rewriting; web UI action; progress reporting for large transfers.
4. **T4, auto-transfer:** agents report capacity in `/v1/health` or a new `/v1/info` (CPU load, free memory, session count, battery/on-AC). A long-running hub (`clawsh web` or a new `clawsh balance`) moves idle, opted-in sessions from hosts over a threshold to the host with the most headroom, with hysteresis and a cooldown per session so they don't bounce. Never moves a session that is working or has a viewer attached.

## Open questions

- Default workspace mode: is `present` + path mapping enough for the common case, or should `git` be the default?
- Should the source keep its export for a while as an undo (`clawsh move --undo`)?
- Sessions announced via `/v1/external` (not owned by clawsh): support moving them by asking the user to exit claude first?
