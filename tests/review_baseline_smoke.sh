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
  repo="$(mktemp -d /tmp/scafld-review-baseline.XXXXXX)"
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

write_approved_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/approved/baseline-task.md" \
    "baseline-task" "approved" "Review baseline smoke" \
    "app.txt" "grep -q '^changed$' app.txt"
}

strip_workspace_baseline() {
  local repo="$1"
  REPO="$repo" python3 - <<'PY'
import json
import os
import pathlib

repo = pathlib.Path(os.environ["REPO"])
session_path = repo / ".scafld" / "runs" / "baseline-task" / "session.json"
session = json.loads(session_path.read_text())
session.pop("workspace_baseline", None)
session["entries"] = [entry for entry in session.get("entries", []) if entry.get("type") != "workspace_baseline"]
session_path.write_text(json.dumps(session, indent=2) + "\n")
PY
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/4] build a task in a dirty repo with a missing baseline"
(
  cd "$repo"
  printf 'pre-existing dirty file\n' > unowned.txt
  printf 'changed\n' > app.txt
)
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build baseline-task --json"
assert_json "$output" "data['ok'] is True" "build should pass before review"
strip_workspace_baseline "$repo"

echo "[2/4] audit bootstraps a baseline and filters unrelated dirty files"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld audit baseline-task --json"
assert_json "$output" "data['ok'] is True" "audit should ignore pre-existing dirty files after bootstrapping"
assert_json "$output" "data['result']['baseline']['source'] == 'audit_bootstrap'" "audit should record the bootstrap baseline source"
assert_json "$output" "'unowned.txt' not in data['result']['undeclared']" "pre-existing dirty file should not count as scope drift"
assert_json "$output" "'app.txt' in data['result']['matched']" "declared task file should still count as task-owned work"

echo "[3/4] review keeps scope drift green and emits a challenger handoff"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review baseline-task --json"
assert_json "$output" "data['ok'] is True" "review should pass automated checks in a dirty repo"
assert_json "$output" "any(item['id'] == 'scope_drift' and item['result'] == 'pass' for item in data['result']['automated_passes'])" "scope drift should pass after baseline bootstrapping"
assert_json "$output" "data['result']['handoff_role'] == 'challenger'" "review should emit a challenger handoff"
[ -f "$repo/.scafld/runs/baseline-task/handoffs/challenger-review.md" ] || fail "review handoff should exist"

echo "[4/4] session retains the bootstrapped baseline for subsequent review rounds"
REPO="$repo" python3 - <<'PY'
import json
import os
import pathlib

repo = pathlib.Path(os.environ["REPO"])
session = json.loads((repo / ".scafld" / "runs" / "baseline-task" / "session.json").read_text())
baseline = session.get("workspace_baseline") or {}
assert baseline.get("source") == "audit_bootstrap", baseline
assert "unowned.txt" in (baseline.get("paths") or {}), baseline
PY

echo "PASS: review baseline smoke"
