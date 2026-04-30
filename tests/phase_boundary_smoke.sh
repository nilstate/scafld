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
  repo="$(mktemp -d /tmp/scafld-phase-boundary.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    printf 'base\n' > first.txt
    printf 'base\n' > second.txt
    git add .
    git commit -m "init" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

write_approved_multiphase_spec() {
  local repo="$1"
  SPEC_REPO="$repo" PYTHONPATH="$REPO_ROOT" python3 - <<'PY'
import os
from pathlib import Path
from scafld.spec_markdown import render_spec_markdown

repo = Path(os.environ["SPEC_REPO"])
data = {
    "spec_version": "2.0",
    "task_id": "phase-boundary",
    "created": "2026-04-25T00:00:00Z",
    "updated": "2026-04-25T00:00:00Z",
    "status": "approved",
    "harden_status": "not_run",
    "task": {
        "title": "Phase boundary smoke",
        "summary": "Ensure build only executes the current phase",
        "size": "small",
        "risk_level": "low",
        "context": {"cwd": ".", "files_impacted": [{"path": "first.txt", "reason": "fixture"}, {"path": "second.txt", "reason": "fixture"}]},
    },
    "planning_log": [{"timestamp": "2026-04-25T00:00:00Z", "actor": "test", "summary": "Bootstrap phase boundary fixture"}],
    "phases": [
        {
            "id": "phase1",
            "name": "Finish first file",
            "objective": "Only phase1 should run on the first build",
            "changes": [{"file": "first.txt", "action": "update", "content_spec": "Replace base with phase1-done"}],
            "acceptance_criteria": [{"id": "ac1_1", "type": "custom", "description": "first.txt contains phase1-done", "command": "grep -q '^phase1-done$' first.txt", "expected_kind": "exit_code_zero"}],
            "status": "pending",
        },
        {
            "id": "phase2",
            "name": "Finish second file",
            "objective": "Phase2 should wait for a second explicit execution pass",
            "changes": [{"file": "second.txt", "action": "update", "content_spec": "Replace base with phase2-done"}],
            "acceptance_criteria": [{"id": "ac2_1", "type": "custom", "description": "second.txt contains phase2-done", "command": "grep -q '^phase2-done$' second.txt", "expected_kind": "exit_code_zero"}],
            "status": "pending",
        },
    ],
}
path = repo / ".scafld/specs/approved/phase-boundary.md"
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(render_spec_markdown(data), encoding="utf-8")
PY
}

main() {
  local repo output
  repo="$(new_repo)"
  write_approved_multiphase_spec "$repo"

  echo "[1/5] satisfy phase1 only"
  printf 'phase1-done\n' > "$repo/first.txt"

  echo "[2/5] first build only executes phase1"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build phase-boundary --json"
  assert_json "$output" "data['command'] == 'build' and data['ok'] is True" "build should succeed when the current phase passes"
  assert_json "$output" "len(data['result']['criteria']) == 1 and data['result']['criteria'][0]['phase'] == 'phase1'" "build should execute only the current phase criteria"
  assert_json "$output" "data['result']['summary']['passed'] == 1 and data['result']['summary']['failed'] == 0" "phase1 build should record one passing criterion"
  assert_json "$output" "data['result']['next_action']['type'] == 'phase_handoff' and data['result']['next_action']['phase_id'] == 'phase2'" "build should surface the phase2 handoff instead of reviewing immediately"
  assert_json "$output" "data['result']['current_handoff']['selector'] == 'phase2'" "current handoff should advance to phase2"

  echo "[3/5] status keeps phase2 pending after the first build"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status phase-boundary --json"
  assert_json "$output" "data['result']['phase_statuses'][0]['id'] == 'phase1' and data['result']['phase_statuses'][0]['status'] == 'completed'" "status should mark phase1 completed"
  assert_json "$output" "data['result']['phase_statuses'][1]['id'] == 'phase2' and data['result']['phase_statuses'][1]['status'] == 'pending'" "status should leave phase2 pending after the first build"

  echo "[4/5] default exec names the current phase before running it"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec phase-boundary --resume"; then
    fail "phase2 exec should fail before second.txt is updated"
  fi
  assert_contains "$output" "phase: phase2" "human exec output should name the resolved current phase"
  assert_contains "$output" "ac2_1" "default exec should run the phase2 criterion"
  assert_not_contains "$output" "ac1_1" "default exec should not rerun the already-completed phase1 criterion"

  echo "[5/5] json exec names the current phase after the fix is applied"
  printf 'phase2-done\n' > "$repo/second.txt"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec phase-boundary --resume --json"
  assert_json "$output" "data['ok'] is True and data['state']['executed_phase'] == 'phase2'" "json exec should expose the executed phase"
  assert_json "$output" "data['result']['executed_phase'] == 'phase2' and len(data['result']['criteria']) == 1 and data['result']['criteria'][0]['phase'] == 'phase2'" "json exec should execute only phase2 and report it explicitly"

  echo "PASS: phase boundary smoke"
}

main "$@"
