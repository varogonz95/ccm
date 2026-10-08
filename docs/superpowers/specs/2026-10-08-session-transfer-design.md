# Session transfer between machines (`clawsh move`)

Status: idea, design draft. Depends on M3 (hooks) for idle detection and the claude session id.

## Goal

Move a running Claude Code session from one machine to another and keep working there with the full conversation, as if it had started on the target.

Use cases:

- The source machine is short on CPU or memory; move heavy sessions off it.
- Move sessions to a laptop before travelling, so they keep running locally offline.
- Free a machine for a reboot or update without losing context.
- Later: automatic placement on the machine with the most headroom.

Success: `clawsh move pc1/a3f laptop` stops the session on `pc1` and archives it there, and a minute later `clawsh attach laptop/<id>` shows the same conversation, in the same project with the same uncommitted changes, with claude able to keep editing files.

Works for sessions clawsh started and for external sessions (claude started in a plain terminal and announced by the plugin).

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

1. **Check.** The hub asks the target for a dry run (path mapping, workspace, name). Any failure aborts before the source is touched.
1. **Quiesce.** Wait until the session is idle (M3 `Stop` hook: claude finished its turn). Moving mid-turn would cut a tool call in half, so `move` refuses with "session is working" unless `--wait` (block until idle) or `--force`.
2. **Stop.** The source agent ends claude gracefully (`/exit`, then Ctrl-C, then kill), so the last transcript line is flushed. External sessions: see below.
3. **Export.** The source agent packs a tarball: transcript, file-history, todos, a manifest (session id, name, source `Dir`, source home, OS, claude version), and the workspace delta (see below).
4. **Ship.** The hub streams `GET` from the source agent into `PUT` on the target agent. Both endpoints already use the bearer token in `hosts.toml`.
5. **Import and rewrite.** The target agent maps the source `Dir` to a target `Dir`, prepares the workspace, rewrites paths (below), writes files under its own `~/.claude`, and starts `claude --resume <session-id>` in the target `Dir` as a normal clawsh session, named as below.
6. **Archive.** Only after the target session is running does the hub tell the source to archive its session. A failed import leaves the source session stopped but not archived, and `clawsh resume` restarts it there.

### Archived sessions

The source keeps a record instead of deleting anything:

- New status `archived`, with `moved_to` (`laptop/<id>`), the claude session id and the time of the move. The transcript files stay where they are.
- `ls` hides archived sessions; `ls -a` shows them. They cannot be attached (no process), only resumed or removed.
- `clawsh resume pc1/a3f` restarts an archived session on the source with `--resume` (an undo). It warns that the two copies now diverge.
- `clawsh rm pc1/a3f` drops the record. Transcript files are left to Claude Code's own cleanup.
- Sessions don't survive an agent restart today, so the agent persists archived records to a small state file (`<config>/clawsh/archive.json`). First step towards session persistence in "Later".

### Naming on the target

The new session is called `<name> (moved)`. If that name is already used on the target, it becomes `<name> (2)`, `<name> (3)` and so on. A previous ` (moved)` or ` (N)` suffix is stripped first, so moving back and forth does not stack suffixes. `--name` overrides.

### External sessions

Sessions clawsh did not start (announced by `clawsh mcp`, listed under `/v1/external`) can be moved too, with two differences:

- **Hooks come from the plugin.** The plugin ships `hooks.json` with the same `SessionStart` and `Stop` hooks M3 injects, so the agent learns the claude session id, transcript path and idle state of external sessions as well.
- **Stop by signal.** The agent cannot type `/exit` into a terminal it does not own. Once the session is idle it ends the announced claude pid (SIGTERM on Unix, TerminateProcess on Windows). The transcript is already flushed after every message, so nothing is lost; the user's terminal just sees claude exit. The external lease is replaced by an archived record.

After the move the session is a normal clawsh-owned session on the target.

### Why not scp

scp needs SSH between every pair of machines, keys on both sides, and OpenSSH on Windows. The agents already speak authenticated HTTP to the hub, so the hub relays the stream. A direct agent-to-agent path can come later for large workspaces.

## Workspace

The transcript is useless if the target has no copy of the code, and this is the hardest part. Three modes, picked per move:

- **`--workspace=git`** (default): ship what the target can't get from the remote. The source sends its branch and `HEAD` commit id, commits not on any remote as a `git bundle`, and uncommitted plus untracked (non-ignored) files as a tar. On the target:
  - the repo is missing at the mapped path: `git clone` it from the source's `origin` URL (the target needs its own credentials for that remote);
  - the tree there is dirty: refuse, nothing has been stopped yet;
  - otherwise fetch, apply the bundle, check out the same branch at the same commit, and lay the uncommitted files on top.

  The checks that can fail (target path, dirty tree, clone access) run before the source session is stopped. Not a git repo: refuse and suggest `present` or `copy`.
- **`--workspace=present`**: the project already exists on the target and is in sync (shared folders, Syncthing, network drives). Nothing is shipped; clawsh only checks that the path exists.
- **`--workspace=copy`**: tar the whole `Dir`, honouring `.gitignore` when it is a git repo. For projects that aren't git repos. Can be large; a size cap applies (`--max-size`).

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

- `POST /v1/sessions/{id}/stop` `{wait: bool}`: graceful stop, keeps transcript, returns the claude session id. Distinct from `DELETE` (kill). Also accepts an external session id.
- `POST /v1/import/check` (manifest only): dry run on the target (path mapping, dirty tree, clone access, name). Called before `stop`.
- `GET /v1/sessions/{id}/export?workspace=git|present|copy`: streams the tarball. Session must be stopped.
- `PUT /v1/import` (body: tarball, query: target `dir`, name, cols, rows): validates, prepares the workspace, rewrites, writes, resumes; returns the new `api.Session`.
- `POST /v1/sessions/{id}/archive` `{moved_to}`: marks the source session archived.
- `POST /v1/sessions/{id}/resume`: restarts an archived session on its own host.
- `api.Session` gains `status: "archived"`, `moved_to`, `claude_session_id`.

Hub: `clawsh move <host/id> <target-host> [--dir path] [--name n] [--workspace git|present|copy] [--wait|--force]`, `clawsh resume <host/id>`, `clawsh rm <host/id>`, `ls -a`. Web UI: a "Move to..." action on the session card; archived sessions in a collapsed section.

## Safety

- The executable stays fixed by `clawsh agent --claude`; import only adds `--resume <id>`.
- Import validates every tar entry: no absolute paths, no `..`, no symlinks out of the destination, size cap.
- Import writes only under the agent user's `~/.claude` and under the target `Dir`; the target `Dir` must be inside an allowlist (`clawsh agent --workspace-root`, defaults to home). Without that, a token holder could write anywhere the agent user can.
- The workspace tarball can contain secrets (`.env`); `copy` mode warns and honours `.gitignore`. Transfer is plain HTTP on the LAN until TLS lands (see `docs/wiki/Security.md`).

## Phases

1. **T1:** `stop`, `export`, `import` with `git` and `present` workspace modes and path mapping; transcript rewrite with tests; archived sessions with `resume` and `rm`; naming; `clawsh move`. Same-OS only, clawsh-owned sessions only.
2. **T2:** external sessions (plugin hooks, stop by signal); `copy` workspace mode.
3. **T3:** cross-OS rewriting; web UI action; progress reporting for large transfers.
4. **T4, auto-transfer:** agents report capacity in `/v1/health` or a new `/v1/info` (CPU load, free memory, session count, battery/on-AC). A long-running hub (`clawsh web` or a new `clawsh balance`) moves idle, opted-in sessions from hosts over a threshold to the host with the most headroom, with hysteresis and a cooldown per session so they don't bounce. Never moves a session that is working or has a viewer attached.

## Decisions (2026-10-08)

- Default workspace mode is `git`.
- The source session is archived, not deleted; `resume` is the undo.
- Target name is `<name> (moved)`, then `(2)`, `(3)` on collision.
- External sessions can be moved.
