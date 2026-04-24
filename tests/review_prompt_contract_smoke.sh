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
  repo="$(mktemp -d /tmp/scafld-review-contract.XXXXXX)"
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

write_active_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/active/review-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "review-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "in_progress"

task:
  title: "Review prompt contract smoke"
  summary: "Verify the challenger prompt contract and required sections"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap review prompt contract smoke fixture"

phases:
  - id: "phase1"
    name: "Keep the marker"
    objective: "app.txt should stay ok"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "keep the marker green"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "app.txt contains ok"
        command: "grep -q '^ok$' app.txt"
        expected: "exit code 0"
        result: "pass"
    status: "completed"
EOF
}

repo="$(new_repo)"
printf 'ok\n' > "$repo/app.txt"
write_active_spec "$repo"

echo "[1/2] source review prompt carries the stricter contract"
assert_contains_file "$REPO_ROOT/.ai/prompts/review.md" 'Required finding format' "review prompt should teach the required finding format"
assert_contains_file "$REPO_ROOT/.ai/prompts/review.md" 'No issues found — checked <what you attacked>' "review prompt should teach the explicit no-issues format"

echo "[2/2] review scaffolding emits the required sections and format guidance"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review review-task --json"
assert_json "$output" "'Regression Hunt' in data['result']['required_sections']" "review should require the regression hunt section"
assert_json "$output" "'Blocking' in data['result']['required_sections']" "review should require the blocking section"
assert_contains "$output" 'path/file.py:88' "review prompt should carry the strict finding example"
assert_contains "$output" '<what you attacked>' "review prompt should carry the explicit no-issues guidance"

echo "PASS: review prompt contract smoke"
