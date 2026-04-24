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
  mkdir -p "$repo/.ai/specs/$dir"
  cat > "$repo/.ai/specs/$dir/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "$status"
harden_status: "not_run"

task:
  title: "Retire $task_id"
  summary: "Exercise direct cancellation from $status"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap retirement fixture"

phases:
  - id: "phase1"
    name: "No-op"
    objective: "No-op"
    changes:
      - file: "noop.txt"
        action: "update"
        lines: "1"
        content_spec: "no-op"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "noop"
        command: "printf 'ok\\n'"
        expected: "exit code 0"
    status: "pending"
EOF
}

repo="$(new_repo)"

echo "[1/2] approved work can be cancelled without entering runtime"
write_draft_spec "$repo" "approved-task" "approved" "approved"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel approved-task"
assert_contains "$output" "status: cancelled" "cancel should archive an approved spec directly"
[ -f "$repo/.ai/specs/archive/2026-04/approved-task.yaml" ] || fail "approved spec should move to archive when cancelled"

echo "[2/2] drafts can be cancelled directly"
write_draft_spec "$repo" "draft-task" "draft" "drafts"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel draft-task"
assert_contains "$output" "status: cancelled" "cancel should archive a draft directly"
[ -f "$repo/.ai/specs/archive/2026-04/draft-task.yaml" ] || fail "draft spec should move to archive when cancelled"

echo "PASS: retire approved smoke"
