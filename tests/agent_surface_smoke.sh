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
  repo="$(mktemp -d /tmp/scafld-agent-surface.XXXXXX)"
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

write_mixed_markers() {
  local repo="$1"
  (
    cd "$repo"
    mkdir -p tests
    cat > package.json <<'EOF'
{
  "name": "agent-surface-mixed",
  "packageManager": "npm@11.0.0",
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "vitest run",
    "lint": "eslint .",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^19.0.0"
  }
}
EOF
    : > package-lock.json
    cat > tsconfig.json <<'EOF'
{
  "compilerOptions": {
    "strict": true
  }
}
EOF
    cat > pyproject.toml <<'EOF'
[project]
name = "agent-surface-mixed"
version = "0.1.0"

[tool.pytest.ini_options]
addopts = "-q"
EOF
    : > uv.lock
  )
}

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/agent-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "agent-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Agent surface smoke"
  summary: "Exercise plan/build wrapper commands"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap agent surface smoke fixture"

phases:
  - id: "phase1"
    name: "Write the marker"
    objective: "agent.txt should end up green"
    changes:
      - file: "agent.txt"
        action: "update"
        lines: "1"
        content_spec: "replace red with green"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "agent.txt contains green"
        command: "grep -q '^green$' agent.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

repo="$(new_repo)"
write_mixed_markers "$repo"

echo "[1/5] default help exposes only the slim agent surface"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld --help"
assert_contains "$output" "init" "default help should expose init"
assert_contains "$output" "plan" "default help should expose plan"
assert_contains "$output" "build" "default help should expose build"
assert_contains "$output" "exec" "default help should expose exec"
assert_contains "$output" "review" "default help should expose review"
assert_not_contains "$output" "start" "default help should hide start"
assert_not_contains "$output" "adapter" "default help should hide the internal adapter command"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld --help --advanced"
assert_contains "$output" "harden" "advanced help should expose harden"
assert_contains "$output" "audit" "advanced help should expose audit"
assert_contains "$output" "exec" "advanced help should expose exec"
assert_not_contains "$output" "start" "advanced help should not expose removed start"
assert_not_contains "$output" "adapter" "advanced help should hide the internal adapter command"

echo "[2/5] plan creates a draft and opens harden"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld plan draft-task -t 'Draft task' -s small -r low --json"
assert_json "$output" "data['state']['status'] == 'draft'" "plan should create a draft spec"
assert_json "$output" "data['state']['harden_status'] == 'in_progress'" "plan should open harden"
assert_json "$output" "data['result']['mark_passed_command'] == 'scafld harden draft-task --mark-passed'" "plan should return the harden completion command"
assert_json "$output" "data['result']['repo_context']['summary'] == 'Mixed repo detected: Node (npm), React, TypeScript + Python (uv)'" "plan should surface mixed repo detection"
assert_contains_file "$repo/.ai/specs/drafts/draft-task.yaml" 'command: npm run build && uv run python -m compileall .' "plan draft should carry the mixed compile command"
assert_contains_file "$repo/.ai/specs/drafts/draft-task.yaml" 'command: npm test && uv run pytest' "plan draft should carry the mixed test command"

echo "[3/5] plan reopens harden on an existing draft"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld plan draft-task -t 'Draft task' -s small -r low --json"
assert_json "$output" "data['result']['reused_existing_draft'] is True" "plan should reuse an existing draft"
assert_json "$output" "data['state']['harden_status'] == 'in_progress'" "plan should reopen harden for an existing draft"
assert_json "$output" "data['state']['round'] == 2" "plan should open a new harden round when reused"

echo "[4/5] build starts approved work and immediately runs exec"
write_approved_spec "$repo"
if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build agent-task --json"; then
  fail "first build should fail the acceptance criterion and emit recovery"
fi
assert_json "$output" "data['state']['action'] == 'start_exec'" "first build should start and exec in one call"
assert_json "$output" "data['result']['initial_handoff']['role'] == 'executor'" "build should expose the initial executor handoff"
assert_json "$output" "data['result']['initial_handoff']['gate'] == 'phase'" "build should expose the initial phase gate"
assert_json "$output" "data['result']['exec']['next_action']['type'] == 'recovery_handoff'" "first build should emit a recovery handoff when validation fails"
assert_json "$output" "data['result']['next_action']['type'] == 'recovery_handoff'" "build should expose the canonical next action"
assert_json "$output" "data['result']['current_handoff']['gate'] == 'recovery'" "build should expose the current recovery handoff"

echo "[5/5] build advances in-progress work through exec"
printf 'green\n' > "$repo/agent.txt"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build agent-task --json"
assert_json "$output" "data['state']['action'] == 'exec'" "second build should run exec"
assert_json "$output" "data['ok'] is True" "build should succeed after the file is fixed"
assert_json "$output" "data['result']['next_action']['type'] == 'review'" "passing build should point at review"

echo "PASS: agent surface smoke"
