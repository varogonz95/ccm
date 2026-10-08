---
name: Agent-ready task
about: A refined issue an AI agent can implement end to end without asking questions
labels: agent-ready
---

<!--
Definition of ready (all must hold before the `agent-ready` label replaces `needs_refinement`):
- Fits in one PR. Larger work is a parent issue with phased sub-issues.
- No open questions: every decision is written down below.
- Every acceptance criterion can be checked by a command or a test.
- Dependencies are listed; the agent skips the issue while any is open.
-->

## Goal

One or two sentences: what changes for the user and why.

## Context

- Parent / related issues, spec or design doc links.
- Files and packages involved (`internal/...`), and the existing code to follow as a pattern.

## Scope

In:
- ...

Out (do not do in this issue):
- ...

## Decisions

Choices already made (names, flags, defaults, wire formats), so the agent doesn't have to pick.

## Rules and limits

- Repo rules in `CLAUDE.md` apply (cross-platform build, fixed executable, single websocket writer, minimal deps, `internal/api` types first).
- Issue-specific constraints: ...

## Steps

1. ...

## Acceptance criteria

- [ ] Behavior: ...
- [ ] Tests: ... (name the test files)
- [ ] `make test` and `make vet` pass.
- [ ] Docs updated: `docs/wiki/...`, `docs/PLAN.md`.

## Manual checks

Anything an agent can't verify (Windows, browser UI): list it, and say it goes in the PR description for a human.

## Dependencies

- Blocked by: #...
