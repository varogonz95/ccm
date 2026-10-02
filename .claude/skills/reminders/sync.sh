#!/usr/bin/env bash
# Sync reminders/ with the `reminders` branch on origin.
#
#   sync.sh pull                  fetch origin/reminders and bring new/updated files down
#   sync.sh push "<message>"      pull, then commit local reminder files and push to origin/reminders
#   sync.sh status                show what the local dir and the branch each have
#
# The `reminders` branch is an orphan branch holding only reminders/, checked out in a
# dedicated worktree so the main checkout's branch and working tree are never touched.
# Pull never overwrites a local file that has diverged — it reports a conflict and leaves both
# copies in place for the caller to merge.

set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BRANCH="reminders"
WT="$ROOT/.claude/worktrees/reminders"
LOCAL="$ROOT/reminders"
REMOTE_DIR="$WT/reminders"

log() { printf '%s\n' "$*"; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

sum() { [ -f "$1" ] && md5sum "$1" | cut -d' ' -f1 || echo "-"; }

ensure_worktree() {
  mkdir -p "$LOCAL"
  if [ ! -e "$WT/.git" ]; then
    git -C "$ROOT" fetch --quiet origin "$BRANCH":"refs/remotes/origin/$BRANCH" 2>/dev/null
    if git -C "$ROOT" show-ref --verify --quiet "refs/remotes/origin/$BRANCH"; then
      git -C "$ROOT" worktree add --quiet -B "$BRANCH" "$WT" "origin/$BRANCH" \
        || die "could not create the reminders worktree at $WT"
    else
      # First run ever: start `reminders` as an orphan branch with an empty tree.
      local empty commit
      empty="$(git -C "$ROOT" hash-object -t tree /dev/null)"
      commit="$(git -C "$ROOT" commit-tree "$empty" -m "Initialize reminders branch")" \
        || die "could not create the initial reminders commit"
      git -C "$ROOT" branch "$BRANCH" "$commit" || die "could not create branch $BRANCH"
      git -C "$ROOT" worktree add --quiet "$WT" "$BRANCH" \
        || die "could not create the reminders worktree at $WT"
      log "created a new orphan \`$BRANCH\` branch (origin had none)"
    fi
  fi
  mkdir -p "$REMOTE_DIR"
}

# Fetch the branch and fast-forward the worktree, remembering what each file looked like
# beforehand so pull can tell "local is in sync" from "local has diverged".
declare -A BEFORE
fetch_branch() {
  local f
  for f in "$REMOTE_DIR"/*.md; do
    [ -e "$f" ] || continue
    BEFORE["$(basename "$f")"]="$(sum "$f")"
  done

  git -C "$ROOT" fetch --quiet origin "$BRANCH":"refs/remotes/origin/$BRANCH" 2>/dev/null
  if git -C "$ROOT" show-ref --verify --quiet "refs/remotes/origin/$BRANCH"; then
    if ! git -C "$WT" merge --quiet --ff-only "origin/$BRANCH" 2>/dev/null; then
      log "warning: \`$BRANCH\` worktree could not fast-forward to origin/$BRANCH — resolve in $WT"
      return 1
    fi
  else
    log "note: origin has no \`$BRANCH\` branch yet; it will be created on the first push"
  fi
  return 0
}

pull() {
  ensure_worktree
  fetch_branch
  local conflicts=0 pulled=0 f name l r
  for f in "$REMOTE_DIR"/*.md; do
    [ -e "$f" ] || continue
    name="$(basename "$f")"
    l="$(sum "$LOCAL/$name")"
    r="$(sum "$f")"
    if [ ! -f "$LOCAL/$name" ]; then
      cp "$f" "$LOCAL/$name"; pulled=$((pulled + 1)); log "pulled  $name (new)"
    elif [ "$l" = "$r" ]; then
      :
    elif [ "$l" = "${BEFORE[$name]:-}" ]; then
      cp "$f" "$LOCAL/$name"; pulled=$((pulled + 1)); log "pulled  $name (updated)"
    elif [ "$r" = "${BEFORE[$name]:-}" ]; then
      # Only the local copy changed; push carries it up.
      :
    else
      conflicts=$((conflicts + 1))
      log "CONFLICT $name — local and branch both changed; branch copy: $f"
    fi
  done
  [ "$pulled" -eq 0 ] && [ "$conflicts" -eq 0 ] && log "up to date"
  [ "$conflicts" -gt 0 ] && return 2
  return 0
}

push() {
  local msg="${1:-Update reminders}" rc=0
  pull || rc=$?
  [ "$rc" -eq 2 ] && die "unresolved conflicts — merge them into the local files before pushing"
  local f
  for f in "$LOCAL"/*.md; do
    [ -e "$f" ] || continue
    cp "$f" "$REMOTE_DIR/$(basename "$f")"
  done
  git -C "$WT" add -A reminders
  if git -C "$WT" diff --cached --quiet; then
    log "nothing to commit"
  else
    git -C "$WT" commit --quiet -m "$msg" \
      || die "commit failed"
    log "committed: $msg"
  fi
  git -C "$WT" push --quiet origin "HEAD:refs/heads/$BRANCH" \
    || die "push to origin/$BRANCH failed"
  log "pushed to origin/$BRANCH"
}

list_md() {
  local f found=0
  for f in "$1"/*.md; do [ -e "$f" ] && { log "  $(basename "$f")"; found=1; }; done
  [ "$found" -eq 0 ] && log "  (none)"
  return 0
}

status() {
  ensure_worktree
  fetch_branch
  log "local  ($LOCAL):"; list_md "$LOCAL"
  log "branch ($BRANCH):"; list_md "$REMOTE_DIR"
  return 0
}

case "${1:-}" in
  pull)   pull ;;
  push)   shift; push "${1:-Update reminders}" ;;
  status) status ;;
  *)      die "usage: sync.sh pull | push \"<message>\" | status" ;;
esac
