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
  local task_id="$2"
  local status="$3"
  mkdir -p "$(dirname "$path")"
  cat > "$path" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-04-21T00:00:00Z"
updated: "2026-04-21T00:00:00Z"
status: "$status"
harden_status: "not_run"

task:
  title: "Smoke $task_id"
  summary: "JSON contract smoke fixture"
  size: "small"
  risk_level: "low"
  context:
    packages:
      - "cli"
    invariants:
      - "domain_boundaries"
  objectives:
    - "Exercise native JSON contracts."
  touchpoints:
    - area: "tests"
      description: "Exercise native JSON contracts."
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Fixture exists."
        status: "pending"
    validation:
      - id: "v1"
        type: "test"
        description: "README exists"
        command: "test -f README.md"
        expected: "exit code 0"

planning_log:
  - timestamp: "2026-04-21T00:00:00Z"
    actor: "test"
    summary: "Fixture created."

phases:
  - id: "phase1"
    name: "Smoke"
    objective: "Exercise native JSON contracts."
    changes:
      - file: "README.md"
        action: "update"
        content_spec: "JSON smoke touches README.md"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "README exists"
        command: "test -f README.md"
        expected: "exit code 0"
    status: "pending"

rollback:
  strategy: "per_phase"
  commands:
    phase1: "git checkout HEAD -- README.md"
EOF
}

WS="$(mktemp -d /tmp/scafld-json-smoke.XXXXXX)"
TMP_DIRS+=("$WS")

echo "[1/10] init --json emits a stable envelope"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init --json"
assert_json "$output" "data['ok'] is True and data['command'] == 'init'" "init --json should emit a success envelope"
assert_json "$output" "data['result']['config']['path'] == '.ai/config.local.yaml'" "init --json should describe config.local creation"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m init >/dev/null 2>&1)

echo "[2/10] new/status/validate --json emit native task state"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld new json-template --json"
assert_json "$output" "data['command'] == 'new' and data['task_id'] == 'json-template'" "new --json should emit task identity"
write_spec "$WS/.ai/specs/drafts/json-flow.yaml" "json-flow" "draft"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status json-flow --json"
assert_json "$output" "data['command'] == 'status' and data['state']['status'] == 'draft'" "status --json should emit state"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate json-flow --json"
assert_json "$output" "data['command'] == 'validate' and data['result']['valid'] is True" "validate --json should emit validation result"

echo "[3/10] approve/start --json emit transitions"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve json-flow --json"
assert_json "$output" "data['command'] == 'approve' and data['result']['transition']['status'] == 'approved'" "approve --json should emit transition data"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld start json-flow --json"
assert_json "$output" "data['command'] == 'start' and data['state']['status'] == 'in_progress'" "start --json should emit active state"

printf 'json smoke\n' > "$WS/README.md"

echo "[4/10] exec --json emits per-criterion results"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec json-flow --json"
assert_json "$output" "data['command'] == 'exec' and data['result']['summary']['failed'] == 0" "exec --json should report zero failures"
assert_json "$output" "data['result']['criteria'][0]['status'] == 'pass'" "exec --json should emit criterion statuses"

echo "[5/10] audit --json emits declared/matched/missing structure"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit json-flow --json"
assert_json "$output" "data['command'] == 'audit' and data['result']['counts']['undeclared'] == 0" "audit --json should report clean scope"
assert_json "$output" "'README.md' in data['result']['matched']" "audit --json should surface matched files"

echo "[6/10] fail --json emits archive transition"
write_spec "$WS/.ai/specs/active/json-fail.yaml" "json-fail" "in_progress"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld fail json-fail --json"
assert_json "$output" "data['command'] == 'fail' and data['state']['status'] == 'failed'" "fail --json should emit failed status"

echo "[7/10] cancel --json emits archive transition"
write_spec "$WS/.ai/specs/active/json-cancel.yaml" "json-cancel" "in_progress"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld cancel json-cancel --json"
assert_json "$output" "data['command'] == 'cancel' and data['state']['status'] == 'cancelled'" "cancel --json should emit cancelled status"

echo "[8/10] report --json aggregates machine-readable stats"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['command'] == 'report' and data['result']['total_specs'] >= 4" "report --json should emit aggregate totals"
assert_json "$output" "'completed' in data['result']['by_status'] or 'failed' in data['result']['by_status']" "report --json should emit status buckets"

echo "[9/10] top-level errors use structured error envelopes"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status missing-task --json"; then
  fail "missing status --json should fail"
fi
assert_json "$output" "data['ok'] is False and data['error']['code'] == 'spec_not_found'" "status --json should emit structured spec_not_found errors"

echo "[10/10] report/update path stays parseable after prior transitions"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld report --json"
assert_json "$output" "data['result']['triage']['review_drift'] == []" "report --json should keep triage collections machine-readable"

echo "PASS: json contract smoke"
