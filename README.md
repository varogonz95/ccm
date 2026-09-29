# ccm — Claude Code Manager

See and drive every Claude Code session on every machine in your LAN from one terminal.

```
ccm ls
TARGET         NAME      STATUS   VIEWERS  AGE    DIR
desk/815856a6  api       running  0        2h14m  /home/alvaro/src/api
laptop/3fa0c1  frontend  running  1        12m    C:\src\frontend
```

One binary, two roles:

- **agent**: runs on each machine, owns `claude` processes inside PTYs (ConPTY on Windows), keeps them alive while nobody is watching, and serves a small REST + WebSocket API.
- **hub**: the `ls / new / attach / kill` commands, run from whichever terminal you're in.

## Setup

Requires Go 1.22+.

```sh
go mod tidy          # first time only
make build           # or: go build -o ccm ./cmd/ccm
make dist            # cross-compile all platforms into dist/
```

On **each machine that should host sessions**:

```sh
ccm agent            # listens on :7420, creates a token on first run
ccm token            # prints the token
```

Windows: allow `ccm.exe` through Windows Defender Firewall (private networks) when prompted.

On **the machine you drive from**, copy `hosts.example.toml` to the path shown by `ccm help`, fill in each agent's URL and token, then:

```sh
ccm hosts                                   # who's reachable
ccm new desk --dir ~/src/api                # start claude there and attach
ccm new desk --dir ~/src/api -- --resume    # pass args through to claude
ccm ls                                      # everything, everywhere
ccm attach desk/8158                        # id prefix is enough
                                            # Ctrl-] detaches, session keeps running
ccm kill desk/8158
```

## Security

Anyone holding an agent's token can run Claude Code on that machine, with that machine's permissions. Traffic is plain HTTP, so keep agents on a trusted LAN until TLS/Tailscale support lands (see `docs/PLAN.md`). The command an agent runs is fixed by `--claude` on the agent; the API only accepts extra args.

## Known limits (MVP)

- Only manages sessions started through ccm, not `claude` launched in a plain terminal.
- Sessions stop when the agent stops. `claude --resume` recovers the conversation.
- Status is `running` / `exited`. Working / idle / needs-input arrives with M3.
- Windows: killing a session doesn't yet kill claude's child processes (Job Object TODO).
