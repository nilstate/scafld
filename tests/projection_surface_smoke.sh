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

assert_json() {
  local payload="$1"
  local expression="$2"
  local message="$3"
  JSON_PAYLOAD="$payload" python3 - "$expression" "$message" <<'PY' || fail "$message"
import json
import os
import sys

expression = sys.argv[1]
message = sys.argv[2]
data = json.loads(os.environ["JSON_PAYLOAD"])
if not eval(expression, {"__builtins__": {}}, {"data": data}):
    raise SystemExit(message)
PY
}

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

write_spec() {
  local path="$1"
  cat > "$path" <<'EOF'
spec_version: "1.1"
task_id: "projection-flow"
created: "2026-04-21T00:00:00Z"
updated: "2026-04-21T00:00:00Z"
status: "draft"

task:
  title: "Projection Flow"
  summary: "Project spec state into markdown and JSON surfaces for PRs and CI."
  size: "medium"
  risk_level: "medium"
  context:
    packages:
      - "cli"
    invariants:
      - "spec_remains_the_single_source_of_truth"
  objectives:
    - "Render a concise summary."
    - "Render a deterministic PR body."
  risks:
    - description: "Projection output could drift from the source spec."
  touchpoints:
    - area: "tests"
      description: "Projection surface smoke."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Projection commands render."
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "tracked file exists"
        command: "test -f tracked.txt"
        expected: "exit code 0"

planning_log:
  - timestamp: "2026-04-21T00:00:00Z"
    actor: "test"
    summary: "Fixture created."

phases:
  - id: "phase1"
    name: "Projection"
    objective: "Render projection surfaces."
    changes:
      - file: "tracked.txt"
        action: "update"
        content_spec: "Projection smoke mutates tracked.txt to induce drift."
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "tracked file exists"
        command: "test -f tracked.txt"
        expected: "exit code 0"
        result: "pass"
        executed_at: "2026-04-21T00:01:00Z"
        result_output: "ok"
    status: "completed"

rollback:
  strategy: "manual"
  commands:
    phase1: "git checkout HEAD -- tracked.txt"
EOF
}

write_review() {
  local path="$1"
  local reviewed_head="$2"
  cat > "$path" <<EOF
# Review: projection-flow

## Review 1 — 2026-04-21T00:05:00Z

### Metadata

\`\`\`json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "",
  "reviewed_at": "2026-04-21T00:05:00Z",
  "reviewed_head": "$reviewed_head",
  "reviewed_dirty": false,
  "reviewed_diff": "projection-smoke",
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass_with_issues",
    "dark_patterns": "pass"
  }
}
\`\`\`

### Pass Results

- Spec Compliance: PASS
- Scope Drift: PASS
- Regression Hunt: PASS
- Convention Check: PASS WITH ISSUES
- Dark Patterns: PASS

### Regression Hunt

No regressions found.

### Convention Check

- Non-blocking: keep projection wording terse.

### Dark Patterns

No issues found.

### Blocking

None

### Non-blocking

- projection copy should stay compact.

### Verdict

pass_with_issues
EOF
}

WS="$(mktemp -d /tmp/scafld-projection-smoke.XXXXXX)"
TMP_DIRS+=("$WS")

echo "[1/6] init workspace, baseline commit, and active fixture"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init >/dev/null"
printf 'seed\n' > "$WS/tracked.txt"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m "chore: seed workspace" >/dev/null 2>&1)
write_spec "$WS/.ai/specs/drafts/projection-flow.yaml"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve projection-flow >/dev/null && PATH='$CLI_ROOT':\"\$PATH\" scafld start projection-flow >/dev/null && PATH='$CLI_ROOT':\"\$PATH\" scafld branch projection-flow >/dev/null"
write_review "$WS/.ai/reviews/projection-flow.md" "$(git -C "$WS" rev-parse HEAD)"

echo "[2/6] summary renders aligned markdown and JSON"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow"
assert_contains "$markdown" "## scafld: Projection Flow" "summary should render the task title"
assert_contains "$markdown" 'Review: `pass_with_issues`' "summary should render the review verdict"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow --json"
assert_json "$output" "data['command'] == 'summary' and data['result']['model']['title'] == 'Projection Flow'" "summary --json should expose the model title"
assert_json "$output" "'## scafld: Projection Flow' in data['result']['markdown']" "summary --json markdown should match the human surface"

echo "[3/6] checks --json emits CI-friendly success when review and sync are clean"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld checks projection-flow --json"
assert_json "$output" "data['command'] == 'checks' and data['result']['check']['status'] == 'success'" "checks --json should succeed for a clean reviewed spec"
assert_json "$output" "'review pass_with_issues' in data['result']['check']['summary']" "checks --json should summarize the review state"

echo "[4/6] pr-body renders deterministic workflow markdown"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld pr-body projection-flow"
assert_contains "$markdown" "# Projection Flow" "pr-body should render the title heading"
assert_contains "$markdown" "## Workflow State" "pr-body should render workflow state"
assert_contains "$markdown" "## Objectives" "pr-body should include objectives"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld pr-body projection-flow --json"
assert_json "$output" "data['command'] == 'pr-body' and '## Workflow State' in data['result']['markdown']" "pr-body --json should return the markdown body"

echo "[5/6] checks fail structurally when engineering drift appears"
printf 'dirty\n' >> "$WS/tracked.txt"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld checks projection-flow --json"; then
  fail "checks should fail when engineering drift exists"
fi
assert_json "$output" "data['error']['code'] == 'projection_check_failed' and data['state']['check_status'] == 'failure'" "checks should emit a structured projection failure"
assert_json "$output" "'workspace has uncommitted changes' in data['result']['model']['sync']['reasons']" "checks should surface sync drift reasons directly"

echo "[6/6] summary reflects drift in markdown too"
capture markdown bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld summary projection-flow"
assert_contains "$markdown" 'Sync: `drift`' "summary markdown should show sync drift"

echo "PASS: projection surface smoke"
