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
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/approved/adapter-task.md" \
    "adapter-task" "approved" "Claude adapter smoke" \
    "adapter.txt" "grep -q '^green$' adapter.txt"
}

write_review_ready_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
  SCAFLD_SPEC_PHASE_STATUS="completed" \
  SCAFLD_SPEC_CRITERION_RESULT="pass" \
    write_markdown_spec "$repo/.scafld/specs/active/review-task.md" \
    "review-task" "in_progress" "Claude review adapter smoke" \
    "review.txt" "grep -q '^ok$' review.txt"
}

repo="$(new_repo)"
stub_dir="$(write_provider_stub)"
write_approved_spec "$repo"

echo "[1/3] init seeds the claude adapter scripts"
[ -x "$repo/.scafld/core/scripts/scafld-provider-adapter.sh" ] || fail "init should seed the shared provider adapter"
[ -x "$repo/.scafld/core/scripts/scafld-claude-build.sh" ] || fail "init should seed the claude build adapter script"
[ -x "$repo/.scafld/core/scripts/scafld-claude-review.sh" ] || fail "init should seed the claude review adapter script"

echo "[2/3] approved work feeds the executor handoff to claude"
capture_path="$repo/claude-phase.txt"
args_path="$repo/claude-phase.args"
(
  cd "$repo"
  PATH="$stub_dir:$CLI_ROOT:$PATH" \
  SCAFLD_ADAPTER_CAPTURE="$capture_path" \
  SCAFLD_ADAPTER_ARGS="$args_path" \
  ./.scafld/core/scripts/scafld-claude-build.sh adapter-task --phase-arg >/dev/null
)
assert_contains_file "$capture_path" 'role: "executor"' "claude adapter should feed an executor handoff"
assert_contains_file "$capture_path" '## Phase Objective' "claude adapter should feed the phase handoff content"
assert_contains_file "$args_path" '--phase-arg' "claude adapter should pass through provider args"
assert_contains_file "$repo/.scafld/runs/adapter-task/session.json" '"type": "provider_invocation"' "claude adapter should record provider invocation telemetry"
assert_contains_file "$repo/.scafld/runs/adapter-task/session.json" '"role": "executor"' "claude adapter should record executor attribution"
assert_contains_file "$repo/.scafld/runs/adapter-task/session.json" '"provider": "claude"' "claude adapter should record the claude provider"
assert_contains_file "$repo/.scafld/runs/adapter-task/session.json" '"confidence": "unknown"' "claude adapter should record attribution confidence"

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
  ./.scafld/core/scripts/scafld-claude-review.sh review-task >/dev/null
)
assert_contains_file "$capture_path" 'role: "challenger"' "claude adapter should feed a challenger handoff for review-ready work"
assert_contains_file "$capture_path" '## Challenge Contract' "claude adapter should feed the review challenge contract"
assert_contains_file "$review_repo/.scafld/runs/review-task/session.json" '"role": "challenger"' "claude review adapter should record challenger attribution"
assert_contains_file "$review_repo/.scafld/runs/review-task/session.json" '"provider": "claude"' "claude review adapter should record the claude provider"

echo "PASS: claude handoff adapter smoke"
