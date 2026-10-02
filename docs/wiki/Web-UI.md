# Web UI

`ccm web` opens a browser page that shows every machine in `hosts.toml` and its Claude Code sessions. You can open a session in a full-page terminal, start new ones and end them, without typing other commands.

## Start it

```
ccm web
```

It prints a link like `http://127.0.0.1:7421/?k=…` and opens it in your default browser. Keep the terminal open; Ctrl-C stops the web UI (your sessions keep running on their machines).

| Flag | Default | |
|---|---|---|
| `--listen` | `127.0.0.1:7421` | Loopback address to serve on. Other addresses are refused. |
| `--no-open` | off | Print the link without opening a browser |
| `--config` | `<config dir>/ccm/hosts.toml` | Hosts file |

## What you see

- **Your machines:** a card per host with its sessions. Offline machines show why. The page updates by itself.
- **A session:** the terminal fills the page. Leaving it only disconnects you; Claude keeps working.
- **New session:** pick a machine, a folder and optionally a name and extra Claude arguments.
- **No machines yet:** the steps to add one. The page notices when you save `hosts.toml`.

## Limits

- Opens only on the computer running `ccm web` (see [Security](Security.md)). Phones and other devices: issue #6.
- Editing machines from the page: issue #8.
- Dark theme only.
