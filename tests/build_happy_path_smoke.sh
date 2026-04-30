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
  repo="$(mktemp -d /tmp/scafld-build-happy.XXXXXX)"
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

write_approved_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/approved/happy-task.md" \
    "happy-task" "approved" "Build happy path smoke" \
    "happy.txt" "grep -q '^green$' happy.txt"
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/5] approved work points at build plus the current phase handoff"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status happy-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'build'" "approved status should point at build"
assert_json "$output" "data['result']['current_handoff']['gate'] == 'phase'" "approved status should expose the current phase handoff"
assert_json "$output" "data['result']['current_handoff']['selector'] == 'phase1'" "approved status should point at phase1"

echo "[2/5] first build fails into a recovery handoff"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build happy-task --json"; then
  fail "first build should fail and emit recovery guidance"
fi
assert_json "$output" "data['state']['action'] == 'start_exec'" "approved build should start and exec in one call"
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "first build should point at recovery"
assert_json "$output" "data['result']['current_handoff']['gate'] == 'recovery'" "first build should expose the current recovery handoff"

echo "[3/5] status mirrors the recovery guidance"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status happy-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "status should mirror build recovery guidance"
assert_json "$output" "data['result']['current_handoff']['selector'] == 'ac1_1'" "status should point at the failing criterion"

echo "[4/5] a passing build points at review"
printf 'green\n' > "$repo/happy.txt"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build happy-task --json"
assert_json "$output" "data['ok'] is True" "second build should pass"
assert_json "$output" "data['result']['next_action']['type'] == 'review'" "passing build should point at review"

echo "[5/5] status mirrors the review-ready state"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status happy-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'review'" "status should point at review once execution is complete"

echo "PASS: build happy path smoke"
