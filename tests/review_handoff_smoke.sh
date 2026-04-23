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
  repo="$(mktemp -d /tmp/scafld-review-handoff.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    git add .
    git commit -m "bootstrap" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/review-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "review-task"
created: "2026-04-23T00:00:00Z"
updated: "2026-04-23T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Review handoff smoke"
  summary: "Exercise review handoff generation"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-23T00:00:00Z"
    actor: "user"
    summary: "Bootstrap review handoff smoke fixture"

phases:
  - id: "phase1"
    name: "Write the review marker"
    objective: "app.txt should end up changed"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "replace base with changed"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "app.txt contains changed"
        command: "grep -q '^changed$' app.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/3] build runs a passing phase"
(
  cd "$repo"
  printf 'changed\n' > app.txt
)
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build review-task --json"
assert_json "$output" "data['ok'] is True" "build should pass before review"
assert_json "$output" "data['state']['action'] == 'start_exec'" "build should activate approved work in one call"

echo "[2/3] review emits a fresh review handoff"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review review-task --json"
assert_json "$output" "data['result']['handoff_file'] == '.ai/runs/review-task/handoffs/challenger-review.md'" "review should return the challenger handoff path"
assert_json "$output" "data['result']['handoff_json_file'] == '.ai/runs/review-task/handoffs/challenger-review.json'" "review should return the challenger handoff json path"
assert_json "$output" "data['result']['handoff_role'] == 'challenger'" "review should identify the handoff role"
assert_json "$output" "data['result']['handoff_gate'] == 'review'" "review should identify the handoff gate"
assert_json "$output" "'ADVERSARIAL REVIEW' in data['result']['review_prompt']" "review prompt should come from the review handoff template"
[ -f "$repo/.ai/runs/review-task/handoffs/challenger-review.md" ] || fail "review handoff file should exist"
[ -f "$repo/.ai/runs/review-task/handoffs/challenger-review.json" ] || fail "review handoff json should exist"
[ -f "$repo/.ai/reviews/review-task.md" ] || fail "review artifact should exist"

echo "[3/3] review metadata records the handoff reference"
assert_contains_file "$repo/.ai/reviews/review-task.md" '"review_handoff": ".ai/runs/review-task/handoffs/challenger-review.md"' "review metadata should reference the handoff"
assert_contains_file "$repo/.ai/reviews/review-task.md" '"reviewer_mode": "challenger"' "review metadata should identify the challenger mode"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld handoff review-task --review --json"
assert_json "$output" "data['state']['role'] == 'challenger'" "handoff --review should report the challenger role"
assert_json "$output" "data['state']['gate'] == 'review'" "handoff --review should report the review gate"
assert_json "$output" "data['result']['handoff_file'] == '.ai/runs/review-task/handoffs/challenger-review.md'" "handoff --review should regenerate the same path"
assert_json "$output" "data['result']['handoff_json_file'] == '.ai/runs/review-task/handoffs/challenger-review.json'" "handoff --review should regenerate the same json path"

echo "PASS: review handoff smoke"
