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
  repo="$(mktemp -d /tmp/scafld-claude-adapter.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    git add .
    git commit -m "bootstrap" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

write_provider_stub() {
  local stub_dir
  stub_dir="$(mktemp -d /tmp/scafld-claude-stub.XXXXXX)"
  TMP_DIRS+=("$stub_dir")
  cat > "$stub_dir/claude" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
cat > "${SCAFLD_ADAPTER_CAPTURE:?}"
printf '%s\n' "$*" > "${SCAFLD_ADAPTER_ARGS:?}"
EOF
  chmod +x "$stub_dir/claude"
  printf '%s\n' "$stub_dir"
}

write_approved_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/approved/adapter-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "adapter-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "approved"
harden_status: "not_run"

task:
  title: "Claude adapter smoke"
  summary: "Verify the claude wrapper consumes the current handoff"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap claude adapter smoke fixture"

phases:
  - id: "phase1"
    name: "Write the marker"
    objective: "adapter.txt should end up green"
    changes:
      - file: "adapter.txt"
        action: "update"
        lines: "1"
        content_spec: "replace red with green"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "adapter.txt contains green"
        command: "grep -q '^green$' adapter.txt"
        expected: "exit code 0"
    status: "pending"
EOF
}

write_review_ready_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/active/review-task.yaml" <<'EOF'
spec_version: "1.1"
task_id: "review-task"
created: "2026-04-24T00:00:00Z"
updated: "2026-04-24T00:00:00Z"
status: "in_progress"

task:
  title: "Claude review adapter smoke"
  summary: "Verify the claude wrapper consumes the challenger handoff"
  size: "small"
  risk_level: "low"

planning_log:
  - timestamp: "2026-04-24T00:00:00Z"
    actor: "user"
    summary: "Bootstrap claude review adapter smoke fixture"

phases:
  - id: "phase1"
    name: "Keep the marker"
    objective: "review.txt should stay ok"
    changes:
      - file: "review.txt"
        action: "update"
        lines: "1"
        content_spec: "keep the marker ok"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "review.txt contains ok"
        command: "grep -q '^ok$' review.txt"
        expected: "exit code 0"
        result: "pass"
    status: "completed"
EOF
}

repo="$(new_repo)"
stub_dir="$(write_provider_stub)"
write_approved_spec "$repo"

echo "[1/3] init seeds the claude adapter scripts"
[ -x "$repo/scripts/scafld-provider-adapter.sh" ] || fail "init should seed the shared provider adapter"
[ -x "$repo/scripts/scafld-claude-build.sh" ] || fail "init should seed the claude build adapter script"
[ -x "$repo/scripts/scafld-claude-review.sh" ] || fail "init should seed the claude review adapter script"

echo "[2/3] approved work feeds the executor handoff to claude"
capture_path="$repo/claude-phase.txt"
args_path="$repo/claude-phase.args"
(
  cd "$repo"
  PATH="$stub_dir:$CLI_ROOT:$PATH" \
  SCAFLD_ADAPTER_CAPTURE="$capture_path" \
  SCAFLD_ADAPTER_ARGS="$args_path" \
  ./scripts/scafld-claude-build.sh adapter-task --phase-arg >/dev/null
)
assert_contains_file "$capture_path" 'role: "executor"' "claude adapter should feed an executor handoff"
assert_contains_file "$capture_path" '## Phase Objective' "claude adapter should feed the phase handoff content"
assert_contains_file "$args_path" '--phase-arg' "claude adapter should pass through provider args"

echo "[3/3] review-ready work feeds the challenger handoff to claude"
review_repo="$(new_repo)"
review_stub_dir="$(write_provider_stub)"
write_review_ready_spec "$review_repo"
printf 'ok\n' > "$review_repo/review.txt"
capture_path="$review_repo/claude-review.txt"
args_path="$review_repo/claude-review.args"
(
  cd "$review_repo"
  PATH="$review_stub_dir:$CLI_ROOT:$PATH" \
  SCAFLD_ADAPTER_CAPTURE="$capture_path" \
  SCAFLD_ADAPTER_ARGS="$args_path" \
  ./scripts/scafld-claude-review.sh review-task >/dev/null
)
assert_contains_file "$capture_path" 'role: "challenger"' "claude adapter should feed a challenger handoff for review-ready work"
assert_contains_file "$capture_path" '## Challenge Contract' "claude adapter should feed the review challenge contract"

echo "PASS: claude handoff adapter smoke"
