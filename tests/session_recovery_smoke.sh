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
  repo="$(mktemp -d /tmp/scafld-session-recovery.XXXXXX)"
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

write_approved_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-23T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/approved/recovery-task.md" \
    "recovery-task" "approved" "Recovery smoke" \
    "demo.txt" "grep -q '^green$' demo.txt"
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/4] build initializes session, phase handoff, and the first recovery"
printf 'red\n' > "$repo/demo.txt"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build recovery-task --json"; then
  fail "first build should fail before the file is fixed"
fi
assert_json "$output" "data['state']['action'] == 'start_exec'" "build should start and execute in one call"
assert_json "$output" "data['result']['start']['session_file'] == '.scafld/runs/recovery-task/session.json'" "build should report the session file"
assert_json "$output" "data['result']['initial_handoff']['handoff_file'].endswith('executor-phase-phase1.md')" "build should report the first phase handoff"
assert_json "$output" "data['result']['initial_handoff']['role'] == 'executor'" "build should report the handoff role"
assert_json "$output" "data['result']['initial_handoff']['gate'] == 'phase'" "build should report the handoff gate"
[ -f "$repo/.scafld/runs/recovery-task/handoffs/executor-phase-phase1.json" ] || fail "phase handoff json should exist after build"

echo "[2/4] failing build writes diagnostics and a recovery handoff"
assert_json "$output" "data['error']['code'] == 'acceptance_failed'" "failing build should return acceptance_failed"
assert_json "$output" "len(data['result']['exec']['recovery_handoffs']) == 1" "failing build should emit one recovery handoff"
[ -f "$repo/.scafld/runs/recovery-task/session.json" ] || fail "session file should exist after exec"
[ -f "$repo/.scafld/runs/recovery-task/diagnostics/ac1_1-attempt1.txt" ] || fail "diagnostic file should exist after failure"
[ -f "$repo/.scafld/runs/recovery-task/handoffs/executor-recovery-ac1_1-1.md" ] || fail "recovery handoff should exist after failure"
[ -f "$repo/.scafld/runs/recovery-task/handoffs/executor-recovery-ac1_1-1.json" ] || fail "recovery handoff json should exist after failure"

echo "[3/4] passing build records a phase summary"
printf 'green\n' > "$repo/demo.txt"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build recovery-task --json"
assert_json "$output" "data['ok'] is True" "second build should pass"
assert_json "$output" "data['state']['action'] == 'exec'" "second build should resume execution"
assert_json "$output" "'phase1' in data['result']['summary']['completed_phases']" "passing build should record a completed phase"

echo "[4/4] session ledger reflects failure then recovery"
REPO="$repo" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import json
import os
import pathlib
import sys

repo = pathlib.Path(os.environ["REPO"])
sys.path.insert(0, os.environ["REPO_ROOT"])

session = json.loads((repo / ".scafld" / "runs" / "recovery-task" / "session.json").read_text())
assert session["recovery_attempts"]["ac1_1"] == 1, session
assert len(session["attempts"]) == 2, session
assert session["attempts"][0]["status"] == "fail", session
assert session["attempts"][1]["status"] == "pass", session
assert session["phase_summaries"][0]["phase_id"] == "phase1", session
assert session["workspace_baseline"]["source"] == "start", session
assert session["entries"][0]["type"] == "workspace_baseline", session
assert any(entry["type"] == "attempt" for entry in session["entries"]), session
assert any(entry["type"] == "phase_summary" for entry in session["entries"]), session
PY

echo "PASS: session recovery smoke"
