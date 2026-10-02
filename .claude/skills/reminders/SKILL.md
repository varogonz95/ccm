---
name: reminders
description: Log or recall deferred ccm work and follow-ups. Use automatically, without being asked, at the end of any session or task that yields or suggests follow-up work — follow-up issues, deferred fixes, out-of-scope findings, next steps, things the user said to leave for later — and whenever the user asks what's been deferred or pending. Plain-text log at reminders/REMINDER-<yyyy-mm-dd>.md, synced to the `reminders` branch (pull at session start, commit+push on every new entry), separate from Claude's own memory.
---

# Reminders

A project-local log of follow-up work, distinct from Claude's auto-memory (which is for durable
facts and preferences, not day-to-day work items).

## Trigger — automatic

Before wrapping up a session or task, check whether it produced any of:

- Something the user deferred ("leave X for later", "skip that for now").
- A follow-up you suggested: next step, follow-up issue/PR, a manual Windows check still to run.
- An out-of-scope finding you noticed but didn't fix.
- Work left incomplete (a skipped step, a check not run, an uncleared blocker).

If yes, write one entry per follow-up, push, and say so in one line ("logged 2 follow-ups to
reminders"). Don't log things finished in the same session, work the user is actively doing, or
vague musings with no concrete next action. A follow-up that also becomes a GitHub issue still gets
an entry — note the issue number.

## Where entries go

`reminders/REMINDER-<yyyy-mm-dd>.md` at the repo root (gitignored on `main`). One file per day;
append newest at the bottom.

```markdown
# Reminders — <yyyy-mm-dd>

Deferred items and session follow-ups. Newest entry at the bottom.

## <issue number or short label>
<What it is, where it came from (deferral / suggested next step / out-of-scope finding / left
incomplete), why it wasn't done now, and what to check when picking it up.>
```

A short paragraph each — enough to pick the thread up cold.

## Syncing — the `reminders` branch

Versioned on an orphan `reminders` branch of this repo — never on `main`, never through a PR.
`sync.sh` keeps it checked out in `.claude/worktrees/reminders`, so the main checkout is never
touched:

```bash
bash .claude/skills/reminders/sync.sh pull
bash .claude/skills/reminders/sync.sh push "Add status-hooks follow-up"
bash .claude/skills/reminders/sync.sh status
```

- **Pull once at session start**, silently, the first time this skill runs — before reading or
  writing anything.
- **Push on every new entry**, right after writing it, with a one-line message naming what was
  logged. Don't batch, don't ask. `push` pulls first. No Claude attribution in the commit.
- **Conflict** (`pull` exits 2 and prints `CONFLICT <file> — ... branch copy: <path>`): merge both
  copies into the local file by hand (union, date order), then push. Never drop entries.
- **Sync fails** (no network, or the session may only push its own branch): one plain line to the
  user — the entry is saved locally and the next push carries it. Don't retry in a loop and never
  fall back to committing on `main`.

## Reading

Only when the user asks what's deferred/pending. Read the relevant day files (glob
`reminders/REMINDER-*.md` for a sweep) and summarize each entry in plain language. Don't surface the
log unprompted.
