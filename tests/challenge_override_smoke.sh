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
  repo="$(mktemp -d /tmp/scafld-challenge-override.XXXXXX)"
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
match = list(re.finditer(r"```json\s*\n(.*?)\n```", text, re.DOTALL))[-1]
metadata = json.loads(match.group(1))
metadata.update(state)
review_path.write_text(text[:match.start(1)] + json.dumps(metadata, indent=2) + text[match.end(1):])
PY
}

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/override-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "override-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Challenge override smoke"
  summary: "Exercise review challenge override metrics"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap challenge override smoke fixture"

phases:
  - id: "phase1"
    name: "Write the marker"
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

write_blocking_review() {
  local repo="$1"
  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/override-task.md" <<'EOF'
# Review: override-task

## Spec
Challenge override smoke
Challenge override smoke fixture

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
  "review_handoff": ".ai/runs/override-task/handoffs/challenger-review.md",
  "reviewer_isolation": "fresh_context_handoff",
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "fail",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: FAIL
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
- **high** `app.txt:1` — downstream caller contract broken.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked obvious failure modes.

### Blocking
- **high** `app.txt:1` — downstream caller contract broken.

### Non-blocking
None.

### Verdict
fail
EOF
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/5] build a passing task and render the challenger handoff"
(
  cd "$repo"
  printf 'changed\n' > app.txt
)
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build override-task --json"
assert_json "$output" "data['ok'] is True" "build should pass before review"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld handoff override-task --review --json"
assert_json "$output" "data['state']['role'] == 'challenger'" "review handoff should use the challenger role"
[ -f "$repo/.ai/runs/override-task/handoffs/challenger-review.md" ] || fail "challenger review handoff should exist"

echo "[2/5] override is rejected until a challenger round exists"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete override-task --human-reviewed --reason 'manual audit'"; then
  fail "complete should reject a human override before challenger review exists"
fi
assert_contains "$output" "cannot override before a completed challenger review exists" "complete should require a completed challenger round before override"

echo "[3/5] blocking review can only close through human override"
write_blocking_review "$repo"
inject_review_git_state "$repo" "override-task"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete override-task"; then
  fail "complete should block on a failing challenger verdict"
fi
assert_contains "$output" "latest review failed" "complete should explain the blocking challenger verdict"

echo "[4/5] override records challenge and override entries in session"
capture output bash -lc "cd '$repo' && printf '%s\n' 'override-task' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" scafld complete '\''override-task'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
assert_contains "$output" "override applied" "complete should report the human override"
REPO="$repo" python3 - <<'PY'
import json
import os
import pathlib

repo = pathlib.Path(os.environ["REPO"])
session = json.loads((repo / ".ai" / "runs" / "archive" / "2026-04" / "override-task" / "session.json").read_text())
assert any(entry["type"] == "challenge_verdict" and entry["blocked"] is True for entry in session["entries"]), session
assert any(entry["type"] == "human_override" and entry["gate"] == "review" for entry in session["entries"]), session
PY

echo "[5/5] report exposes challenge_override_rate"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['result']['llm_runtime']['challenge_override_rate']['overrides'] == 1" "report should count human overrides"
assert_json "$output" "data['result']['llm_runtime']['challenge_override_rate']['total'] == 1" "report should count blocked challenges"
assert_json "$output" "data['result']['llm_runtime']['per_task']['override-task']['challenge_override_rate']['rate'] == 1.0" "per-task report should expose challenge_override_rate"

echo "PASS: challenge override smoke"
