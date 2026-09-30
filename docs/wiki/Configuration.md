# Configuration

ccm keeps its files in the OS user config directory (Go's `os.UserConfigDir`):

| OS | Directory |
|---|---|
| Linux | `$XDG_CONFIG_HOME/ccm` or `~/.config/ccm` |
| macOS | `~/Library/Application Support/ccm` |
| Windows | `%AppData%\ccm` |

## `hosts.toml` (hub)

Lists the agents the hub talks to. Override the path with `--config`.

```toml
[[host]]
name = "desk"
url = "http://192.168.1.20:7420"
token = "paste-token-from-desk"

[[host]]
name = "laptop"
url = "http://192.168.1.31:7420"
token = "paste-token-from-laptop"
```

- `name` and `url` are required; names must be unique. The name is what you type in `ccm new desk` and `desk/<id>`.
- A trailing `/` on `url` is ignored.
- `token` is the output of `ccm token` on that machine.

A template lives at `hosts.example.toml` in the repo.

## `agent.token` (agent)

A 64-character hex token, generated on the agent's first run and written with mode `0600`. Delete it and restart the agent to rotate; then update `hosts.toml` everywhere. Use `--token-file` to keep it elsewhere.

## Session environment

Every session is started with the agent's environment plus:

- `CCM_SESSION_ID`: the session's ID
- `TERM=xterm-256color`, `COLORTERM=truecolor` (not on Windows)
