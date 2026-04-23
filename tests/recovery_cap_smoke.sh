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
  repo="$(mktemp -d /tmp/scafld-recovery-cap.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    cat >> .ai/config.local.yaml <<'EOF'
llm:
  recovery:
    max_attempts: 1
EOF
  )
  printf '%s\n' "$repo"
}

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/cap-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "cap-task"
created: "2026-04-23T00:00:00Z"
updated: "2026-04-23T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Recovery cap smoke"
  summary: "Exercise recovery exhaustion behavior"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-23T00:00:00Z"
    actor: "user"
    summary: "Bootstrap recovery cap smoke fixture"

phases:
  - id: "phase1"
    name: "Keep failing"
    objective: "cap.txt still fails twice"
    changes:
      - file: "cap.txt"
        action: "update"
        lines: "1"
        content_spec: "replace red with green"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "cap.txt contains green"
        command: "grep -q '^green$' cap.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/3] first failure emits one recovery handoff"
(
  cd "$repo"
  scafld_cmd start cap-task >/dev/null
  printf 'red\n' > cap.txt
)
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec cap-task --json"; then
  fail "first exec should fail"
fi
assert_json "$output" "data['error']['code'] == 'acceptance_failed'" "first failure should be a normal acceptance failure"
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "first failure should point to a recovery handoff"
assert_json "$output" "len(data['result']['recovery_handoffs']) == 1" "first failure should emit one recovery handoff"

echo "[2/3] second failure exhausts recovery and requires a human"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec cap-task --resume --json"; then
  fail "second exec should still fail"
fi
assert_json "$output" "data['error']['code'] == 'recovery_exhausted'" "second failure should exhaust recovery"
assert_json "$output" "data['result']['summary']['failed_exhausted'] == 1" "summary should count exhausted criteria"
assert_json "$output" "data['result']['next_action']['type'] == 'human_required'" "exhausted recovery should require a human"
assert_json "$output" "len(data['result']['recovery_handoffs']) == 0" "no new recovery handoff should be emitted after the cap"

echo "[3/3] session records failed_exhausted state"
REPO="$repo" python3 - <<'PY'
import json
import os
import pathlib

repo = pathlib.Path(os.environ["REPO"])
session = json.loads((repo / ".ai" / "runs" / "cap-task" / "session.json").read_text())
assert session["criterion_states"]["ac1_1"]["status"] == "failed_exhausted", session
assert session["phases"][0]["blocked_reason"] == "recovery exhausted for ac1_1", session
PY

echo "PASS: recovery cap smoke"
