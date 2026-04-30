#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-retire-approved.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
  )
  printf '%s\n' "$repo"
}

write_draft_spec() {
  local repo="$1"
  local task_id="$2"
  local status="$3"
  local dir="$4"
  mkdir -p "$repo/.scafld/specs/$dir"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/$dir/$task_id.md" \
    "$task_id" "$status" "Retire $task_id" "noop.txt" "printf 'ok\\n'"
}

repo="$(new_repo)"

echo "[1/2] approved work can be cancelled without entering runtime"
write_draft_spec "$repo" "approved-task" "approved" "approved"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel approved-task"
assert_contains "$output" "status: cancelled" "cancel should archive an approved spec directly"
[ -f "$repo/.scafld/specs/archive/2026-04/approved-task.md" ] || fail "approved spec should move to archive when cancelled"

echo "[2/2] drafts can be cancelled directly"
write_draft_spec "$repo" "draft-task" "draft" "drafts"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel draft-task"
assert_contains "$output" "status: cancelled" "cancel should archive a draft directly"
[ -f "$repo/.scafld/specs/archive/2026-04/draft-task.md" ] || fail "draft spec should move to archive when cancelled"

echo "PASS: retire approved smoke"
