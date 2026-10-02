---
name: explain-pr
description: Explain or describe a ccm pull request end to end — match it to its GitHub issue, explain the problem and how the PR solves it, verify it actually does, run a full code review (correctness, races, platform builds, security) with the installed review skills, then either offer to post combined feedback (sorted low-to-critical) as a PR review or offer to approve. Use when the user says "explain this PR", "describe PR #12", "what does this PR do", or asks to review/summarize a specific PR by number or URL — not for a bare "review this diff" with no PR.
---

# Explain PR

Read-only until the final step; nothing is posted or approved without an explicit yes.

GitHub access: the GitHub MCP tools (`mcp__github__*`) when available, otherwise `gh`. Repo:
`varogonz95/ccm` unless the URL says otherwise.

## 1. Identify the PR

- URL → parse owner/repo/number.
- Number (`#12`, `PR 12`) → this repo.
- "this PR" / nothing → the PR for the current branch.

Fetch metadata (title, body, head/base, author, draft, commits, files) and the full diff. If it's a
draft, say so and continue.

## 2. Match the issue

Find the issue number, first match wins: `closes|fixes|resolves #N` in the body, the branch's
middle segment (`feat/<N>/...`), commit messages, any `#N` in the title/body. Fetch the issue with
its comments. No issue → ask whether to proceed review-only or take a number.

## 3. Explain

In plain language: what the issue asks for (body + comments, not just the title), and how the diff
addresses it — map changes to specific asks. Ground it in the diff, not the PR description.

## 4. Verify

Check each acceptance point against the diff: confirmed, partial, or missing. Partial/missing are
findings. For bugs, check the diff fixes the root cause, not a symptom. Also check the repo rules:
`internal/api` types + `e2e_test.go` case for new endpoints, wiki/PLAN.md updated, builds on all
three OSes (`make vet` on the PR head if the diff touches `ptyx`, `console_*` or syscalls).

## 5. Review

Run the `code-review` skill (high effort) and the `security-review` skill against the PR's diff.
Collect findings verbatim, add step 4's, dedupe by file:line. Large diff → fan out review
dimensions to subagents in parallel, then merge into one list.

## 6. Report and gate

One list, **low → critical** (critical last, right before the decision). One line each:
`path:line: [SEVERITY] problem. Fix: suggestion.`

- Findings → ask whether to post them as a PR review comment (never request-changes).
- Clean → say so and ask whether to approve.

Wait for the answer before posting or approving. No AI attribution in anything posted.
