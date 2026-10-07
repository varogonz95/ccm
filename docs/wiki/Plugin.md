# Claude Code plugin (experimental)

The `clawsh` plugin makes every Claude Code session on a machine do two things:

- start a `clawsh agent` if none is running there;
- show up in `clawsh ls` with status `external`, even when claude was started in a plain terminal.

It registers one MCP server, `clawsh mcp`. That server exposes no tools, so it adds nothing to the model's context. Background and findings: [`docs/poc-auto-agent.md`](../poc-auto-agent.md).

## Install

`clawsh` must be on the `PATH` that Claude Code sees.

```
/plugin marketplace add varogonz95/clawsh
/plugin install clawsh@clawsh
```

Or, for one session from a checkout: `claude --plugin-dir ./plugin`.

## Configure

Edit the `args` in `plugin/.mcp.json`. Flags of `clawsh mcp`:

| Flag | Default | |
|---|---|---|
| `--listen` | `:7420` | Where to look for the agent. Also passed to a spawned agent. Use `127.0.0.1:7420` to keep it off the LAN |
| `--claude` | `claude` | `--claude` for a spawned agent |
| `--token-file` | `<config dir>/clawsh/agent.token` | Token used to announce. Also passed to a spawned agent |
| `--log-file` | `<config dir>/clawsh/agent.log` | Spawned agent's output |
| `--no-spawn` | off | Only announce; never start an agent |
| `--no-announce` | off | Only start an agent; don't list the session |

## Behaviour

- The spawned agent is detached. It keeps running after the session that started it ends. Stop it like any agent process.
- Announcements are 90 s leases renewed every 30 s. They're withdrawn when the session ends (Claude Code sends SIGINT). A crashed session disappears from `ls` once its lease expires.
- Sessions started by clawsh itself aren't announced twice.
- External sessions can't be attached or killed through clawsh.
- Logs go to stderr. Claude Code keeps them in its MCP logs (`claude --debug`).
