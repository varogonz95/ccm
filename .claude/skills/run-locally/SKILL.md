---
name: run-locally
description: 'Builds ccm on a given branch (default: current) and runs a local agent + hub loop against it, with /bin/sh (or a given program) standing in for claude, in an isolated config dir so the real hosts.toml is never touched. Use when asked to run, try or demo a branch locally, or to confirm a change works in the real binary rather than only in tests.'
argument-hint: '[branch] [--claude <program>] [--listen :7420]'
allowed-tools: Bash, Read
---

# run-locally

## 0. Branch

1. Target branch: positional arg, default the current branch.
2. If switching: `git status`; stash dirty work with `git stash push -u` (never discard), then
   `git fetch origin && git checkout <branch>`.

## 1. Build

```bash
make build
```

## 2. Isolated config

Both `hosts.toml` and the agent token live under `os.UserConfigDir()`. Point it at a scratch dir so
the user's real config and token are never touched:

```bash
export XDG_CONFIG_HOME="$(mktemp -d)"   # linux
# macOS: UserConfigDir is $HOME/Library/Application Support, so override HOME instead
```

## 3. Agent

Start in the background, default program `/bin/sh` (use `--claude claude` only if the user asks):

```bash
./ccm agent --claude /bin/sh --listen :7420 &> "$XDG_CONFIG_HOME/agent.log" &
```

Wait until `curl -s localhost:7420` answers (poll, don't sleep blindly), then read the token:

```bash
TOKEN=$(./ccm token)
mkdir -p "$XDG_CONFIG_HOME/ccm"
printf '[[host]]\nname = "local"\nurl = "http://localhost:7420"\ntoken = "%s"\n' "$TOKEN" \
  > "$XDG_CONFIG_HOME/ccm/hosts.toml"
```

## 4. Exercise

Run whatever the change needs, e.g. `./ccm hosts`, `./ccm new local`, `./ccm ls`,
`./ccm kill local/<idprefix>`. `attach` needs a real TTY — use `script -qc` or tell the user to run
it themselves. Report the actual output.

## 5. Tear down — every time

Kill the agent you started (by the PID you captured, never by name) and remove the scratch dir.
If something was already listening on the port before step 3, stop and ask before killing anything.
