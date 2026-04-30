#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-real-standard.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    PATH="$CLI_ROOT:$PATH" scafld init >/dev/null
  )
  printf '%s\n' "$repo"
}

write_archived_task() {
  local repo="$1"
  mkdir -p "$repo/.scafld/specs/archive/2026-04"
  mkdir -p "$repo/.scafld/runs/archive/2026-04/demo-task"
  mkdir -p "$repo/.scafld/reviews"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
  SCAFLD_SPEC_PHASE_STATUS="completed" \
  SCAFLD_SPEC_DOD_STATUS="done" \
  SCAFLD_SPEC_CRITERION_RESULT="pass" \
    write_markdown_spec "$repo/.scafld/specs/archive/2026-04/demo-task.md" \
    "demo-task" "completed" "Cohort smoke task" \
    "demo.txt" "grep -q '^ok$' demo.txt"
  cat > "$repo/.scafld/runs/archive/2026-04/demo-task/session.json" <<'EOF'
{
  "schema_version": 3,
  "task_id": "demo-task",
  "created_at": "2026-04-24T00:00:00Z",
  "updated_at": "2026-04-24T00:00:00Z",
  "model_profile": "default",
  "entries": [],
  "recovery_attempts": {},
  "criterion_states": {
    "ac1_1": {
      "status": "pass",
      "phase_id": "phase1",
      "reason": null,
      "updated_at": "2026-04-24T00:00:00Z"
    }
  },
  "phases": [
    {
      "phase_id": "phase1",
      "attempt_count": 1,
      "criterion_ids": ["ac1_1"],
      "completed_at": "2026-04-24T00:00:00Z",
      "blocked_at": null,
      "blocked_reason": null
    }
  ],
  "attempts": [
    {
      "attempt_index": 1,
      "criterion_id": "ac1_1",
      "phase_id": "phase1",
      "criterion_attempt": 1,
      "status": "pass",
      "command": "grep -q '^ok$' demo.txt",
      "expected": "exit code 0",
      "cwd": null,
      "exit_code": 0,
      "output_snippet": "",
      "diagnostic_path": null,
      "recorded_at": "2026-04-24T00:00:00Z"
    }
  ],
  "phase_summaries": [
    {
      "phase_id": "phase1",
      "summary": "phase1 complete",
      "created_at": "2026-04-24T00:00:00Z"
    }
  ],
  "workspace_baseline": null,
  "usage": {}
}
EOF
}

write_review_file() {
  local repo="$1"
  cat > "$repo/.scafld/reviews/demo-task.md" <<'EOF'
# Review: demo-task

## Review 1 — 2026-04-24T00:00:00Z

### Metadata
```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "challenger",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-04-24T00:00:00Z",
  "override_reason": null,
  "reviewed_head": null,
  "reviewed_dirty": null,
  "reviewed_diff": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of demo.txt.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked obvious null and retry paths.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF
}

repo="$(new_repo)"
write_archived_task "$repo"
write_review_file "$repo"

echo "[1/2] cohort script emits task metrics and fixed questions"
capture output bash -lc "cd '$REPO_ROOT' && PATH='$CLI_ROOT':\"\$PATH\" python3 scripts/real_standard.py --root '$repo' --task demo-task --json"
assert_json "$output" "data['tasks'][0]['task_id'] == 'demo-task'" "cohort script should emit the requested task"
assert_json "$output" "data['tasks'][0]['runtime']['first_attempt_pass_rate']['passed'] == 1" "cohort script should surface first-attempt pass data"
assert_json "$output" "data['aggregate']['review_signal']['completed_rounds'] == 1" "cohort script should surface aggregate review signal"
assert_json "$output" "data['tasks'][0]['review_signal']['format_compliant_clean_review'] is True" "cohort script should surface per-task review signal"
assert_json "$output" "data['tasks'][0]['questions'][0]['id'] == 'build_flow'" "cohort script should emit the fixed question set"

echo "[2/2] markdown mode stays readable"
capture output bash -lc "cd '$REPO_ROOT' && PATH='$CLI_ROOT':\"\$PATH\" python3 scripts/real_standard.py --root '$repo' --task demo-task"
assert_contains "$output" "Task Cohort Summary" "markdown mode should render a heading"
assert_contains "$output" "Format-compliant clean reviews" "markdown mode should render review signal metrics"
assert_contains "$output" "Format-compliant clean review" "markdown mode should render per-task review signal metrics"
assert_not_contains "$output" "Clean review with evidence" "markdown mode should not render the stale per-task review signal label"
assert_not_contains "$output" "Clean reviews with evidence" "markdown mode should not render the stale aggregate review signal label"
assert_contains "$output" "Did build feel easier than managing the same task through a raw agent loop?" "markdown mode should render the fixed questions"

echo "PASS: real standard cohort smoke"
