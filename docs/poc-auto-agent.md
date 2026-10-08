# PoC: auto-start the agent from a Claude Code session

**Question.** Can opening a Claude Code session start a `clawsh agent` on that machine (if none runs) and announce the session to it, so clawsh can pull it, using a plugin that registers an MCP server the way Serena does?

**Verdict: feasible.** It works end to end on Linux with Claude Code 2.1.287. The plugin is in `plugin/`, the stub is `clawsh mcp`.

## How it works

1. `plugin/.mcp.json` registers a stdio MCP server, `clawsh mcp`. Claude Code starts it at session start, as it does for any plugin MCP server.
2. `clawsh mcp` answers the MCP handshake with **no capabilities**: no tools, no prompts, no resources, so nothing is added to the model's context. It keeps the connection open for the whole session.
3. In the background it checks `GET /v1/health` on the local agent. If nothing answers it starts `clawsh agent` **detached** (its own session via `setsid` on Unix; `DETACHED_PROCESS | CREATE_NEW_PROCESS_GROUP | CREATE_BREAKAWAY_FROM_JOB` on Windows) and waits up to 5 s for it. Agent output goes to `<config dir>/clawsh/agent.log`.
4. It then announces the session: `PUT /v1/external/{id}` with dir and claude's pid, renewed every 30 s (90 s lease). `clawsh ls` shows it with status `external`.
5. When the session ends, Claude Code sends SIGINT. The stub sends `DELETE /v1/external/{id}` and exits. The agent keeps running for the next session.

If the session was itself started by clawsh (`CLAWSH_SESSION_ID` is set), the stub skips the announcement: the agent already owns it.

## Observed (Claude Code debug log)

```
MCP server "plugin:clawsh:clawsh": Starting connection with timeout of 30000ms
MCP server "plugin:clawsh:clawsh" Server stderr: clawsh mcp: spawned agent pid 8657 (log: …/clawsh/agent.log)
MCP server "plugin:clawsh:clawsh": Successfully connected (transport: stdio) in 52ms
MCP server "plugin:clawsh:clawsh": Connection established with capabilities: {"hasTools":false,"hasPrompts":false,"hasResources":false,…}
…
MCP server "plugin:clawsh:clawsh": Sending SIGINT to MCP server process
MCP server "plugin:clawsh:clawsh": MCP server process exited cleanly
```

During the session:

```
$ clawsh ls
TARGET                  NAME  STATUS    VIEWERS  AGE  DIR
local/aecd9193be8d78aa  proj  external  -        7s   /…/proj
```

After it: the row is gone and the agent is still up. A session launched through that auto-started agent (`clawsh new local -- -p …`) ran and exited 0, so the env filtering below is enough to avoid claude's nested-session refusal.

## Findings and caveats

- **Shutdown is SIGINT, not stdin EOF.** The stub handles both. A crash or SIGKILL skips the withdraw; the lease expires after 90 s.
- **Env leakage.** The stub's env comes from Claude Code and includes per-session variables (`CLAUDECODE`, `CLAUDE_CODE_SESSION_ID`, `CLAUDE_PID`, …). The agent passes its env on to every claude it launches, so the stub strips those before spawning. It's a denylist: a future Claude Code variable could slip through. More broadly, the auto-started agent inherits the environment (PATH, API keys, proxy settings) of whichever session happened to start it.
- **Network exposure.** By default the auto-started agent listens on `:7420`, all interfaces. Opening Claude Code now silently opens a token-protected port that can run claude. Users who only want local bookkeeping should pass `--listen 127.0.0.1:7420` in `.mcp.json`. Making localhost the default for auto-start is worth considering.
- **`clawsh` must be on PATH** for Claude Code. Plugins can't ship platform binaries cleanly. Alternatives: an absolute path in `.mcp.json`, or a `${CLAUDE_PLUGIN_ROOT}` launcher script that downloads a release.
- **Windows is untested.** It compiles and vets. Risks: if Claude Code puts MCP servers in a job object that forbids breakaway, the stub retries without `CREATE_BREAKAWAY_FROM_JOB`, and the agent then dies with the session. Needs the manual check below.
- **Races.** Two sessions starting at once may both spawn. The second agent fails to bind, logs, and exits. Both stubs then find the first agent healthy. Harmless.
- **The stub reads the token file directly** (same machine, same user), creating it if missing so the stub and a fresh agent agree.
- **External sessions are read-only.** `attach` and `kill` on them return "session not found". The agent doesn't own their PTY. Taking them over isn't possible without the user restarting claude under clawsh.
- **Startup cost** is one health check. The handshake isn't blocked by the spawn, so the 30 s MCP timeout doesn't matter. Claude Code shows the server as connected with no tools.
- **Alternative trigger.** A `SessionStart` hook does the same job without a long-lived process, and gets claude's real `session_id` on stdin. But it can't detect session end, so announcing would rely on lease expiry. The MCP approach gives exact lifetime for free. It's also the natural place to add real tools later (e.g. `clawsh_status` for M3).

## Try it

```sh
make build && cp clawsh ~/bin/               # anywhere on PATH
claude --plugin-dir ./plugin              # one-off, or install:
# /plugin marketplace add varogonz95/clawsh
# /plugin install clawsh@clawsh
clawsh ls                                    # in another terminal (hosts.toml pointing at localhost)
```

## Manual check still needed

1. Windows and macOS: open Claude Code with the plugin, with no agent running. Expect the agent to start and `clawsh ls` to show the session.
2. Close Claude Code. Expect the agent to keep running and the row to disappear.
3. Windows: confirm in Task Manager that the agent survives closing the Claude Code terminal window.
