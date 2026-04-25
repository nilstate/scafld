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
  repo="$(mktemp -d /tmp/scafld-acceptance-write.XXXXXX)"
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

write_active_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/active/rewrite-task.yaml" <<'EOF'
spec_version: "1.1"
# keep-me
task_id: "rewrite-task"
created: "2026-04-25T00:00:00Z"
updated: "2026-04-25T00:00:00Z"
status: "in_progress"
harden_status: "in_progress"
task:
  title: "Acceptance writer smoke"
  summary: |-
    Exercise repeated explicit exec result writes.
    Preserve author-authored YAML outside the runtime fields.
  size: "small"
  risk_level: "low"
planning_log:
  - timestamp: "2026-04-25T00:00:00Z"
    actor: "user"
    summary: "Fixture seeded for acceptance writer smoke"
phases:
  - id: "phase1"
    name: "Emit long output"
    objective: "Produce enough output to force YAML folding"
    changes:
      - file: "fixture.txt"
        action: "update"
        content_spec: "fixture"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "Emit long output and pass"
        command: |-
          python3 -c "print('alpha ' * 40)"
        expected: "exit code 0"
    validation:
      - id: "v1"
        type: "custom"
        description: "Nested result shape stays nested"
        command: "true"
        expected: "exit code 0"
        result:
          status: "fail"
          timestamp: "2026-04-25T00:00:00Z"
          output: "old"
    status: "pending"
EOF
}

repo="$(new_repo)"
mkdir -p "$repo/.ai/specs/active"
write_active_spec "$repo"

echo "[1/4] first explicit exec succeeds"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec rewrite-task --phase phase1 --json"
assert_json "$output" "data['ok'] is True and data['result']['criteria'][0]['status'] == 'pass'" "first explicit exec should pass"

echo "[2/4] second explicit exec also succeeds"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec rewrite-task --phase phase1 --json"
assert_json "$output" "data['ok'] is True and data['result']['criteria'][0]['status'] == 'pass'" "second explicit exec should still pass"

echo "[3/4] resulting spec keeps authoring structure and both result shapes"
capture output bash -lc "cd '$repo' && python3 -c \"import yaml; from pathlib import Path; path = Path('.ai/specs/active/rewrite-task.yaml'); text = path.read_text(); assert '# keep-me' in text; assert 'summary: |-' in text; assert 'command: |-' in text; data = yaml.safe_load(text); criterion = data['phases'][0]['acceptance_criteria'][0]; validation = data['phases'][0]['validation'][0]; assert criterion['result'] == 'pass'; assert 'result_output' in criterion; assert isinstance(validation['result'], dict); assert validation['result']['status'] == 'pass'; assert 'executed_at' not in validation; assert 'result_output' not in validation; print('ok')\""
assert_contains "$output" "ok" "active spec should remain valid YAML after repeated exec"

echo "[4/4] packaged validate still passes after repeated exec"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate rewrite-task --json"
assert_json "$output" "data['ok'] is True and data['result']['valid'] is True" "validate should still succeed after repeated exec"

echo "PASS: acceptance result write smoke"
