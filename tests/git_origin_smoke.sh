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
  PYTHONPATH="$REPO_ROOT" SPEC_PATH="$path" python3 - <<'PY'
import os
from pathlib import Path
from scafld.spec_markdown import render_spec_markdown
data = {
    "spec_version": "2.0",
    "task_id": "origin-flow",
    "created": "2026-04-21T00:00:00Z",
    "updated": "2026-04-21T00:00:00Z",
    "status": "draft",
    "harden_status": "not_run",
    "origin": {
        "source": {
            "system": "github",
            "kind": "issue",
            "id": "123",
            "url": "https://github.com/example/project/issues/123",
            "title": "Bind origin metadata",
        },
        "supersession": None,
        "sync": None,
    },
    "task": {
        "title": "Origin Flow",
        "summary": "Exercise branch binding and sync drift detection in a real git workspace.",
        "size": "small",
        "risk_level": "medium",
        "context": {
            "packages": ["cli"],
            "invariants": ["git_mutation_stays_explicit_and_safe"],
            "files_impacted": [
                {"path": "tracked.txt", "reason": "Branch sync smoke mutates tracked.txt to prove drift."},
                {"path": "cli/scafld", "reason": "Fixture declares the CLI surface under test."},
            ],
        },
        "objectives": ["Create a branch binding and surface drift."],
        "touchpoints": [{"area": "tests", "description": "Exercise git-bound task behavior."}],
        "acceptance": {"validation_profile": "strict", "definition_of_done": [{"id": "dod1", "description": "Fixture exists.", "status": "pending"}], "validation": []},
    },
    "planning_log": [{"timestamp": "2026-04-21T00:00:00Z", "actor": "test", "summary": "Fixture created."}],
    "phases": [
        {
            "id": "phase1",
            "name": "Bind branch",
            "objective": "Bind the task to a working branch and report drift.",
            "changes": [
                {"file": "tracked.txt", "action": "update", "content_spec": "Branch sync smoke mutates tracked.txt to prove drift."},
                {"file": "cli/scafld", "action": "update", "content_spec": "Fixture declares the CLI surface under test."},
            ],
            "acceptance_criteria": [{"id": "ac1_1", "type": "test", "description": "tracked file exists", "command": "test -f tracked.txt", "expected_kind": "exit_code_zero"}],
            "status": "pending",
        }
    ],
    "rollback": {"strategy": "manual", "commands": {"phase1": "git checkout HEAD -- tracked.txt"}},
}
path = Path(os.environ["SPEC_PATH"])
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(render_spec_markdown(data), encoding="utf-8")
PY
}

WS="$(mktemp -d /tmp/scafld-git-origin-smoke.XXXXXX)"
TMP_DIRS+=("$WS")

echo "[1/8] init workspace and baseline commit"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld init >/dev/null"
printf 'seed\n' > "$WS/tracked.txt"
(cd "$WS" && git init -b main >/dev/null 2>&1 && git config user.email smoke@example.com && git config user.name "Smoke Test" && git add . && git commit -m "chore: seed workspace" >/dev/null 2>&1)
write_spec "$WS/.scafld/specs/drafts/origin-flow.md"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve origin-flow >/dev/null && PATH='$CLI_ROOT':\"\$PATH\" scafld build origin-flow >/dev/null"

echo "[2/8] branch --json creates a task branch and records origin metadata"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld branch origin-flow --json"
assert_json "$output" "data['command'] == 'branch' and data['result']['action'] == 'created_branch'" "branch --json should create the task branch"
assert_json "$output" "data['result']['origin']['git']['branch'] == 'origin-flow'" "branch --json should record the bound branch"
assert_json "$output" "data['result']['sync']['status'] == 'in_sync'" "fresh branch binding should be in sync"
[ "$(git -C "$WS" branch --show-current)" = "origin-flow" ] || fail "branch command did not checkout origin-flow"

echo "[3/8] human status surfaces source and binding context"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status origin-flow"
assert_contains "$output" "source: github issue #123 - Bind origin metadata" "status should render the source summary for humans"
assert_contains "$output" "url: https://github.com/example/project/issues/123" "status should render the source URL for humans"
assert_contains "$output" "branch: origin-flow  base: main" "status should render the bound branch and base"
assert_contains "$output" "binding: created branch" "status should render the human binding mode"
assert_contains "$output" "sync: in_sync" "status should render sync state for humans"

echo "[4/8] status --json surfaces origin and sync directly"
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld status origin-flow --json"
assert_json "$output" "data['result']['origin']['git']['branch'] == 'origin-flow'" "status --json should expose stored origin.git.branch"
assert_json "$output" "data['result']['sync']['actual']['branch'] == 'origin-flow'" "status --json should expose the live current branch"
assert_json "$output" "data['result']['sync']['status'] == 'in_sync'" "status --json should report in-sync branch state"

echo "[5/8] branch refuses to switch when the engineering worktree is dirty"
printf 'dirty\n' >> "$WS/tracked.txt"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld branch origin-flow --name alternate --json"; then
  fail "branch should fail when switching with a dirty worktree"
fi
assert_json "$output" "data['error']['code'] == 'dirty_worktree'" "branch --json should emit dirty_worktree when switching with code changes"

echo "[6/8] sync --json reports drift for uncommitted engineering changes"
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld sync origin-flow --json"; then
  fail "sync should fail on dirty engineering changes"
fi
assert_json "$output" "data['error']['code'] == 'git_drift' and data['state']['sync_status'] == 'drift'" "sync --json should report git drift"
assert_json "$output" "'workspace has uncommitted changes' in data['result']['sync']['reasons']" "dirty engineering changes should appear in sync drift reasons"

echo "[7/8] sync detects branch mismatch once the engineering tree is clean"
(cd "$WS" && git checkout -- tracked.txt >/dev/null 2>&1 && git checkout main >/dev/null 2>&1)
if capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld sync origin-flow --json"; then
  fail "sync should fail when checked out on the wrong branch"
fi
assert_json "$output" "data['error']['code'] == 'git_drift'" "branch mismatch should still be reported as git_drift"
assert_json "$output" "[reason for reason in data['result']['sync']['reasons'] if 'expected origin-flow' in reason] != []" "sync drift should explain the expected branch"

echo "[8/8] branch --bind-current rebases the spec onto an existing manual branch"
(cd "$WS" && git checkout -b manual-bind >/dev/null 2>&1)
capture output bash -lc "cd '$WS' && PATH='$CLI_ROOT':\"\$PATH\" scafld branch origin-flow --bind-current --json"
assert_json "$output" "data['result']['action'] == 'bound_current'" "bind-current should record the already-checked-out branch"
assert_json "$output" "data['result']['origin']['git']['branch'] == 'manual-bind'" "bind-current should rewrite the stored branch binding"
assert_json "$output" "data['result']['sync']['status'] == 'in_sync'" "bind-current should leave the spec in sync on the manual branch"

echo "PASS: git origin smoke"
