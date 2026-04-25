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

write_spec() {
  local path="$1"
  cat > "$path" <<'EOF'
spec_version: "1.1"
task_id: "runx-surface"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "draft"

task:
  title: "Runx Surface"
  summary: "Exercise the native split scafld surface consumed by governed runx lanes."
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "app.txt"
    invariants:
      - "native_scafld_json_stays_the_source_of_truth"
  objectives:
    - "Create a draft through the native new command."
    - "Move it through start, branch, exec, review, complete, and projections."
  touchpoints:
    - area: "app.txt"
      description: "One bounded fixture file updated during exec."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "app.txt contains the expected changed marker"
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "app.txt contains the expected changed marker"
        command: "grep -q '^changed$' app.txt"
        expected: "exit code 0"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "smoke"
    summary: "Seeded the runx native surface fixture."

phases:
  - id: "phase1"
    name: "Apply fixture change"
    objective: "Write the changed marker so exec can pass."
    changes:
      - file: "app.txt"
        action: "update"
        content_spec: "Replace the seed contents with changed."
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "app.txt contains the expected changed marker"
        command: "grep -q '^changed$' app.txt"
        expected: "exit code 0"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "git checkout -- app.txt"
EOF
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

  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/$task_id.md" <<EOF
# Review: $task_id

## Spec
Runx surface smoke fixture
Runx surface smoke fixture

## Review 1 — 2026-04-24T00:00:00Z

### Metadata
\`\`\`json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "executor",
  "reviewer_session": "",
  "reviewed_at": "2026-04-24T00:00:00Z",
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
No issues found — checked app.txt for the bounded fixture change.

### Convention Check
No issues found — checked the fixture against the documented workflow contract.

### Dark Patterns
No issues found — checked for hidden scope and state drift.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF

  inject_review_git_state "$repo" "$task_id"
}

WS="$(mktemp -d /tmp/scafld-runx-surface-smoke.XXXXXX)"
TMP_DIRS+=("$WS")
TASK_ID="runx-surface"

echo "[1/8] init workspace, repo, and native draft"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init >/dev/null"
printf 'seed\n' > "$WS/app.txt"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m "bootstrap" >/dev/null 2>&1)
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld new '$TASK_ID' -t 'Runx Surface' -s small -r low --json"
assert_json "$output" "data['command'] == 'new' and data['state']['status'] == 'draft' and data['state']['file'].endswith('drafts/runx-surface.yaml')" "new --json should expose the native draft creation surface"
write_spec "$WS/.ai/specs/drafts/$TASK_ID.yaml"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'validate' and data['result']['valid'] is True" "validate --json should accept the runx surface fixture spec"

echo "[2/8] approve, start, and bind the active branch"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'approve' and data['state']['status'] == 'approved' and data['result']['transition']['to'].endswith('approved/runx-surface.yaml')" "approve --json should move the spec into the approved surface"
(cd "$WS" && git checkout -b docs/runx-surface >/dev/null 2>&1)
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld start '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'start' and data['state']['status'] == 'in_progress' and data['result']['transition']['to'].endswith('active/runx-surface.yaml') and data['result']['handoff_file'].endswith('executor-phase-phase1.md')" "start --json should expose the active transition and executor handoff"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld branch '$TASK_ID' --bind-current --json"
assert_json "$output" "data['command'] == 'branch' and data['result']['origin']['git']['branch'] == 'docs/runx-surface'" "branch --json should persist the bound branch for downstream wrappers"

echo "[3/8] exec, status, and audit stay native"
printf 'changed\n' > "$WS/app.txt"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'exec' and data['ok'] is True and data['result']['summary']['passed'] == 1" "exec --json should expose a passing acceptance summary"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'status' and data['result']['origin']['git']['branch'] == 'docs/runx-surface'" "status --json should carry the recorded origin binding"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'audit' and data['ok'] is True and data['result']['counts']['matched'] == 1 and data['result']['counts']['undeclared'] == 0" "audit --json should confirm the bounded changed file set"

echo "[4/8] the old execute alias is gone"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld execute '$TASK_ID' --resume --json"; then
  fail "execute alias should no longer resolve"
fi
assert_contains "$output" "invalid choice" "execute alias should fail as an invalid command"

echo "[5/8] review and complete emit the handoff and archive facts"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'review' and data['ok'] is True and data['result']['review_file'].endswith('runx-surface.md')" "review --json should expose the challenger handoff surface"
write_review_pass "$WS" "$TASK_ID"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'complete' and data['ok'] is True and data['state']['status'] == 'completed' and data['state']['review_verdict'] == 'pass'" "complete --json should expose the completion verdict"
assert_json "$output" "data['result']['archive_path'].endswith('runx-surface.yaml') and data['result']['review_file'].endswith('runx-surface.md') and data['result']['blocking_count'] == 0 and data['result']['non_blocking_count'] == 0" "complete --json should expose the archive and review facts consumed by wrappers"

echo "[6/8] summary projection stays machine-addressable"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'summary' and data['result']['projection']['surface'] == 'engineering_summary' and data['result']['model']['origin']['git']['branch'] == 'docs/runx-surface'" "summary --json should expose the engineering summary projection and origin model"
assert_json "$output" "'## scafld: Runx Surface' in data['result']['markdown']" "summary --json should carry the rendered summary markdown"

echo "[7/8] checks projection stays CI-friendly even when the tree is still dirty"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld checks '$TASK_ID' --json"; then
  fail "checks --json should report drift after completion when the working tree still holds the reviewed change"
fi
assert_json "$output" "data['command'] == 'checks' and data['result']['projection']['surface'] == 'ci_check' and data['state']['check_status'] == 'failure' and data['error']['code'] == 'projection_check_failed'" "checks --json should expose the CI projection surface even on failure"
assert_json "$output" "'workspace has uncommitted changes' in data['result']['model']['sync']['reasons']" "checks --json should carry the drift reason directly"

echo "[8/8] pr-body projection stays pull-request ready"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld pr-body '$TASK_ID' --json"
assert_json "$output" "data['command'] == 'pr-body' and data['result']['projection']['surface'] == 'pull_request_body' and '## Workflow State' in data['result']['markdown']" "pr-body --json should expose the PR projection surface and markdown"

echo "PASS: runx surface smoke"
