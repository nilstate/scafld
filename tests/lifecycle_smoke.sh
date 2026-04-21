#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()

cleanup() {
  if [ "${#TMP_DIRS[@]}" -gt 0 ]; then
    rm -rf "${TMP_DIRS[@]}"
  fi
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

capture() {
  local __var="$1"
  shift
  local _captured
  set +e
  _captured="$("$@" 2>&1)"
  local status=$?
  set -e
  printf -v "$__var" '%s' "$_captured"
  return "$status"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "$message"
  fi
}

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-lifecycle-smoke.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
  )
  printf '%s\n' "$repo"
}

inject_review_git_state() {
  local repo="$1"
  local task_id="$2"
  REVIEW_REPO="$repo" REVIEW_TASK_ID="$task_id" CLI_SCRIPT="$REPO_ROOT/cli/scafld" python3 - <<'PY'
import json
import os
import pathlib
import re
import runpy

repo = pathlib.Path(os.environ["REVIEW_REPO"])
task_id = os.environ["REVIEW_TASK_ID"]
namespace = runpy.run_path(os.environ["CLI_SCRIPT"])
state, error = namespace["capture_review_git_state"](repo, f".ai/reviews/{task_id}.md")
if error:
    raise SystemExit(error)

review_path = repo / ".ai" / "reviews" / f"{task_id}.md"
text = review_path.read_text()
matches = list(re.finditer(r"```json\s*\n(.*?)\n```", text, re.DOTALL))
if not matches:
    raise SystemExit("review metadata JSON block not found")

metadata_match = matches[-1]
metadata = json.loads(metadata_match.group(1))
metadata.update(state)
review_path.write_text(text[:metadata_match.start(1)] + json.dumps(metadata, indent=2) + text[metadata_match.end(1):])
PY
}

write_review_pass() {
  local repo="$1"
  local task_id="$2"
  local changed_file="$3"

  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/$task_id.md" <<EOF
# Review: $task_id

## Spec
Lifecycle smoke $task_id
Lifecycle smoke fixture

## Files Changed
- $changed_file

---

## Review 1 — 2026-04-21T00:00:00Z

### Metadata
\`\`\`json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-04-21T00:00:00Z",
  "override_reason": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
\`\`\`

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of $changed_file.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked obvious failure modes.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF
  inject_review_git_state "$repo" "$task_id"
}

write_spec_fixture() {
  local path="$1"
  local task_id="$2"
  local status="$3"
  local updated="$4"
  local file_name="$5"
  local include_result="${6:-no}"

  local result_block=""
  if [ "$include_result" = "yes" ]; then
    result_block='        result: "pass"'
  fi

  cat > "$path" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "$updated"
updated: "$updated"
status: "$status"

task:
  title: "Fixture $task_id"
  summary: "Lifecycle smoke fixture"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Fixture"
    objective: "Exercise report triage"
    changes:
      - file: "$file_name"
        action: "update"
        lines: "1"
        content_spec: "Fixture"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "Fixture"
        command: "grep -q '^ok$' $file_name"
        expected: "exit code 0"
$result_block

planning_log:
  - timestamp: "$updated"
    actor: "user"
    summary: "Fixture"
EOF
}

main() {
  local repo draft_path active_path archive_path output report_output
  repo="$(new_repo)"

  echo "[1/6] init a clean workspace and baseline commit"
  (
    cd "$repo"
    scafld_cmd init >/dev/null
    printf 'base\n' > demo.txt
    git add .
    git commit -m "bootstrap" >/dev/null 2>&1
  )

  echo "[2/6] create a real draft, make it valid, and validate it"
  (
    cd "$repo"
    scafld_cmd new demo-task -t "Lifecycle smoke" -s small -r low >/dev/null
  )
  draft_path="$repo/.ai/specs/drafts/demo-task.yaml"
  cat > "$draft_path" <<'EOF'
spec_version: "1.1"
task_id: "demo-task"
created: "2026-04-21T00:00:00Z"
updated: "2026-04-21T00:00:00Z"
status: "draft"
harden_status: "not_run"

task:
  title: "Lifecycle smoke"
  summary: "Exercise the end-to-end lifecycle smoke"
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "demo.txt"
    invariants:
      - "domain_boundaries"
  objectives:
    - "Archive a spec through the full lifecycle"
  touchpoints:
    - area: "demo.txt"
      description: "Lifecycle marker file"
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "demo.txt contains done"
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "demo.txt contains done"
        command: "grep -q '^done$' demo.txt"
        expected: "exit code 0"

planning_log:
  - timestamp: "2026-04-21T00:00:00Z"
    actor: "user"
    summary: "Lifecycle smoke fixture"

phases:
  - id: "phase1"
    name: "Mark the demo file complete"
    objective: "Write the lifecycle marker"
    changes:
      - file: "demo.txt"
        action: "update"
        lines: "1"
        content_spec: "Replace base with done"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "demo.txt contains done"
        command: "grep -q '^done$' demo.txt"
        expected: "exit code 0"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "git checkout -- demo.txt"
EOF
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate demo-task"
  assert_contains "$output" "PASS:" "validate should accept the lifecycle spec"

  echo "[3/6] approve, start, and execute the spec"
  (
    cd "$repo"
    scafld_cmd approve demo-task >/dev/null
    scafld_cmd start demo-task >/dev/null
    printf 'done\n' > demo.txt
  )
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec demo-task"
  assert_contains "$output" "1 passed" "exec should record the passing acceptance criterion"

  echo "[4/6] run review and complete the spec"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review demo-task"
  assert_contains "$output" "ADVERSARIAL REVIEW" "review should emit the handoff prompt"
  write_review_pass "$repo" "demo-task" "demo.txt"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete demo-task"
  assert_contains "$output" "review" "complete should print the final review verdict"
  archive_path="$(find "$repo/.ai/specs/archive" -name demo-task.yaml -print | head -n 1)"
  [ -n "$archive_path" ] || fail "complete should archive the lifecycle spec"

  echo "[5/6] create triage fixtures for report output"
  mkdir -p "$repo/.ai/specs/drafts" "$repo/.ai/specs/approved" "$repo/.ai/specs/active"
  write_spec_fixture "$repo/.ai/specs/drafts/stale-draft.yaml" "stale-draft" "draft" "2025-01-01T00:00:00Z" "stale.txt"
  write_spec_fixture "$repo/.ai/specs/approved/waiting-spec.yaml" "waiting-spec" "approved" "2026-04-01T00:00:00Z" "waiting.txt"
  write_spec_fixture "$repo/.ai/specs/active/no-exec.yaml" "no-exec" "in_progress" "2026-04-01T00:00:00Z" "no-exec.txt"
  write_spec_fixture "$repo/.ai/specs/active/review-drift.yaml" "review-drift" "in_progress" "2026-04-01T00:00:00Z" "drift.txt" "yes"
  printf 'ok\n' > "$repo/drift.txt"
  write_review_pass "$repo" "review-drift" "drift.txt"
  printf 'drifted\n' > "$repo/drift.txt"

  echo "[6/6] report stays runnable and surfaces actionable triage"
  capture report_output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report"
  assert_contains "$report_output" "Triage:" "report should include the triage heading"
  assert_contains "$report_output" "Stale drafts (>7d)" "report should include stale draft triage"
  assert_contains "$report_output" "stale-draft" "report should list stale drafts"
  assert_contains "$report_output" "Approved waiting to start" "report should include approved triage"
  assert_contains "$report_output" "waiting-spec" "report should list approved specs waiting to start"
  assert_contains "$report_output" "Active with no exec evidence" "report should include active/no-exec triage"
  assert_contains "$report_output" "no-exec" "report should list active specs without exec evidence"
  assert_contains "$report_output" "Review drift" "report should include review drift triage"
  assert_contains "$report_output" "review-drift" "report should list review drift specs"
  assert_contains "$report_output" "current workspace no longer matches the reviewed git state" "report should explain review drift"

  echo "PASS: lifecycle smoke"
}

main "$@"
