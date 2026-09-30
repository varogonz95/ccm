# ccm wiki

ccm (Claude Code Manager) lets you see and drive every Claude Code session on every machine in your LAN from one terminal.

It is one binary with two roles:

- **agent** runs on each machine that hosts sessions. It owns `claude` processes inside PTYs (ConPTY on Windows), keeps them alive while nobody is watching, and serves a small REST + WebSocket API.
- **hub** is the set of commands you type (`hosts`, `ls`, `new`, `attach`, `kill`) from whichever terminal you're in. It reads a list of agents from `hosts.toml`.

```
$ ccm ls
TARGET         NAME      STATUS   VIEWERS  AGE    DIR
desk/815856a6  api       running  0        2h14m  /home/alvaro/src/api
laptop/3fa0c1  frontend  running  1        12m    C:\src\frontend
```

## Pages

- [Installation](Installation.md): build from source or with Docker
- [Quick start](Quick-Start.md): agents, `hosts.toml`, your first session
- [Commands](Commands.md): every subcommand and flag
- [Configuration](Configuration.md): `hosts.toml`, tokens, file locations
- [How it works](How-It-Works.md): sessions, scrollback, attach
- [Protocol](Protocol.md): the agent's HTTP and WebSocket API
- [Security](Security.md): what a token grants and how to keep it safe
- [Development](Development.md): building, testing, contributing
- [Roadmap and limits](Roadmap.md): what works today and what's next
