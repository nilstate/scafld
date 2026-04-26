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
  repo="$(mktemp -d /tmp/scafld-list-command.XXXXXX)"
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

write_spec_fixture() {
  local repo="$1"
  local rel_path="$2"
  local task_id="$3"
  local status="$4"
  local title="$5"
  local phase_status="$6"

  mkdir -p "$(dirname "$repo/$rel_path")"
  cat > "$repo/$rel_path" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-04-25T00:00:00Z"
updated: "2026-04-25T00:00:00Z"
status: "$status"
harden_status: "in_progress"
task:
  title: "$title"
  summary: "Fixture for list smoke"
  size: "small"
  risk_level: "low"
planning_log:
  - timestamp: "2026-04-25T00:00:00Z"
    actor: "user"
    summary: "Fixture seeded for list smoke"
phases:
  - id: "phase1"
    name: "Fixture phase"
    objective: "Exercise list output"
    changes:
      - file: "fixture.txt"
        action: "update"
        content_spec: "fixture"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "Fixture criterion"
        command: "true"
        expected: "exit code 0"
    status: "$phase_status"
EOF
}

repo="$(new_repo)"

write_spec_fixture "$repo" ".ai/specs/drafts/draft-task.yaml" "draft-task" "draft" "Draft task" "pending"
write_spec_fixture "$repo" ".ai/specs/approved/approved-task.yaml" "approved-task" "approved" "Approved task" "pending"
write_spec_fixture "$repo" ".ai/specs/active/active-task.yaml" "active-task" "in_progress" "Active task" "completed"
write_spec_fixture "$repo" ".ai/specs/archive/2026-04/completed-task.yaml" "completed-task" "completed" "Completed task" "completed"
write_spec_fixture "$repo" ".ai/specs/archive/2026-04/superseded-task.yaml" "superseded-task" "cancelled" "Superseded task" "completed"
cat >> "$repo/.ai/specs/archive/2026-04/superseded-task.yaml" <<'EOF'
superseded_by: "completed-task"
superseded_at: "2026-04-25T01:00:00Z"
superseded_reason: "Replaced by completed-task"
EOF

echo "[1/3] unfiltered list shows lifecycle buckets without crashing"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list"; then
  fail "scafld list should succeed"
fi
assert_not_contains "$output" "Traceback" "list should not crash with a traceback"
assert_contains "$output" "drafts/" "list should show drafts bucket"
assert_contains "$output" "approved/" "list should show approved bucket"
assert_contains "$output" "active/" "list should show active bucket"
assert_contains "$output" "archive/2026-04/" "list should show archive bucket"
assert_contains "$output" "draft-task" "list should include draft task"
assert_contains "$output" "approved-task" "list should include approved task"
assert_contains "$output" "active-task" "list should include active task"
assert_contains "$output" "completed-task" "list should include archived task"
assert_contains "$output" "stale active" "list should mark active specs with all phases complete"
assert_contains "$output" "superseded by completed-task" "list should mark superseded specs"

echo "[2/3] filtered list narrows to the requested bucket"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list active"; then
  fail "scafld list active should succeed"
fi
assert_not_contains "$output" "Traceback" "filtered list should not crash with a traceback"
assert_contains "$output" "active/" "active filter should keep the active bucket"
assert_contains "$output" "active-task" "active filter should include the active task"
assert_not_contains "$output" "draft-task" "active filter should exclude draft task"
assert_not_contains "$output" "approved-task" "active filter should exclude approved task"
assert_not_contains "$output" "completed-task" "active filter should exclude archived task"

echo "[3/3] state filters expose stale active and superseded specs"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list stale-active"; then
  fail "scafld list stale-active should succeed"
fi
assert_contains "$output" "active-task" "stale-active filter should include active done/open specs"
assert_not_contains "$output" "draft-task" "stale-active filter should exclude drafts"

if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list superseded"; then
  fail "scafld list superseded should succeed"
fi
assert_contains "$output" "superseded-task" "superseded filter should include superseded specs"
assert_not_contains "$output" "active-task" "superseded filter should exclude stale-only specs"

echo "PASS: list command smoke"
