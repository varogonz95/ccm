# Quick start

This walks through two machines, `desk` (hosts sessions) and your laptop (drives them). The laptop can host sessions too; just run an agent there as well.

## 1. Start an agent on each host

```sh
ccm agent
```

It listens on `:7420`. On first run it generates a token and logs where it saved it. Print it any time with:

```sh
ccm token
```

## 2. Tell the hub about your agents

On the machine you drive from, run `ccm help` to see the config path (it ends in `ccm/hosts.toml`), then create that file:

```toml
[[host]]
name = "desk"
url = "http://192.168.1.20:7420"
token = "paste-token-from-desk"
```

Add one `[[host]]` block per agent. See [Configuration](Configuration.md).

## 3. Check they're reachable

```sh
ccm hosts
```

Each host shows `ok (...)`, `unreachable: ...`, or `auth failed: ...` (wrong token).

## 4. Start a session

```sh
ccm new desk --dir ~/src/api
```

This starts `claude` in `~/src/api` on `desk` and attaches your terminal to it. Pass arguments to claude after `--`:

```sh
ccm new desk --dir ~/src/api -- --resume
```

## 5. Detach and come back

Press **Ctrl-]** to detach. The session keeps running on `desk`.

```sh
ccm ls                 # every session on every host
ccm attach desk/8158   # an ID prefix is enough
```

When you re-attach, the agent replays recent output and nudges claude to repaint, so the screen comes back clean.

## 6. Stop it

```sh
ccm kill desk/8158
```
