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
  repo="$(mktemp -d /tmp/scafld-status-guidance.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    printf 'base\n' > app.txt
    git add .
    git commit -m "bootstrap" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

inject_review_git_state() {
  local repo="$1"
  local task_id="$2"
  REVIEW_REPO="$repo" REVIEW_TASK_ID="$task_id" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import json
import os
import pathlib
import re
import sys

repo = pathlib.Path(os.environ["REVIEW_REPO"])
task_id = os.environ["REVIEW_TASK_ID"]
sys.path.insert(0, os.environ["REPO_ROOT"])
from scafld.git_state import capture_review_git_state

state, error = capture_review_git_state(repo, f".ai/reviews/{task_id}.md")
if error:
    raise SystemExit(error)

review_path = repo / ".ai" / "reviews" / f"{task_id}.md"
text = review_path.read_text()
matches = list(re.finditer(r"```json\s*\n(.*?)\n```", text, re.DOTALL))
metadata_match = matches[-1]
metadata = json.loads(metadata_match.group(1))
metadata.update(state)
review_path.write_text(text[:metadata_match.start(1)] + json.dumps(metadata, indent=2) + text[metadata_match.end(1):])
PY
}

write_valid_draft_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/drafts/status-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "status-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "draft"
harden_status: "in_progress"
harden_rounds:
  - round: 1
    started_at: "2026-04-24T00:00:00Z"
    outcome: "in_progress"
    questions: []

task:
  title: "Status guidance smoke"
  summary: "Exercise status next-action guidance"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap status guidance smoke fixture"

phases:
  - id: "phase1"
    name: "Write the marker"
    objective: "app.txt should end up ok"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "replace base with ok"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "app.txt contains ok"
        command: "grep -q '^ok$' app.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

write_review_pass() {
  local repo="$1"
  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/status-task.md" <<'EOF'
# Review: status-task

## Spec
Status guidance smoke
Exercise status next-action guidance

## Files Changed
- app.txt

---

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
  "review_handoff": ".ai/runs/status-task/handoffs/challenger-review.md",
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass"
  },
  "reviewed_head": null,
  "reviewed_dirty": null,
  "reviewed_diff": null
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of app.txt.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked hardcodes and null handling in app.txt.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF
  inject_review_git_state "$repo" "status-task"
}

repo="$(new_repo)"

echo "[1/5] a fresh plan exposes harden as the next action"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld plan status-task -t 'Status guidance smoke' -s small -r low --json"
assert_json "$output" "data['state']['harden_status'] == 'in_progress'" "plan should open harden"
write_valid_draft_spec "$repo"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'harden'" "draft status should point at harden when a round is open"

echo "[2/5] approved work points at build"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld harden status-task --mark-passed --json"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve status-task --json"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'build'" "approved status should point at build"

echo "[3/5] failing execution points at recovery"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build status-task --json"; then
  fail "first build should fail into recovery guidance"
fi
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "build should point at recovery"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "status should mirror the recovery state"

echo "[4/5] passing execution points at review"
printf 'ok\n' > "$repo/app.txt"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'review'" "passing build should point at review"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'review'" "status should point at review"

echo "[5/5] a passing challenger round points at complete"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review status-task --json"
write_review_pass "$repo"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status status-task --json"
assert_json "$output" "data['result']['next_action']['type'] == 'complete'" "status should point at complete after a passing review"

echo "PASS: status next-action smoke"
