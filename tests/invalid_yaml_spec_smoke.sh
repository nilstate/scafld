#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

phase="all"
if [ "${1:-}" = "--phase" ]; then
  phase="${2:-}"
fi

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-invalid-yaml-smoke.XXXXXX)"
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

write_invalid_spec() {
  local path="$1"
  local task_id="$2"
  local status="$3"
  cat > "$path" <<EOF
spec_version: '1.1'
task_id: $task_id
created: '2026-04-25T00:00:00Z'
updated: '2026-04-25T00:00:00Z'
status: $status
harden_status: in_progress
task:
  title: Broken spec
  summary: Invalid yaml fixture
  size: small
  risk_level: low
planning_log:
- timestamp: '2026-04-25T00:00:00Z'
  actor: user
  summary: invalid yaml fixture
phases:
- id: phase1
  name: Broken phase
  objective: exercise malformed yaml handling
  changes:
  - file: foo.txt
    action: update
    content_spec: fixture
  acceptance_criteria:
  - id: ac1_1
    type: custom
    description: \`this line is invalid yaml plain scalar\`
    command: true
    expected: exit code 0
  status: pending
EOF
}

run_gate_phase() {
  local repo
  repo="$(new_repo)"
  (cd "$repo" && scafld_cmd plan bad-yaml -t "Bad yaml" -s small -r low --json >/dev/null)
  write_invalid_spec "$repo/.ai/specs/drafts/bad-yaml.yaml" "bad-yaml" "draft"

  echo "[gate 1/2] validate rejects malformed YAML"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate bad-yaml --json"; then
    fail "validate should fail on malformed YAML"
  fi
  assert_not_contains "$output" "Traceback" "validate should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'validation_failed'" "validate should return a structured validation failure"
  assert_json "$output" "'invalid spec document' in ' '.join(data['error']['details'])" "validate details should explain the malformed spec"

  echo "[gate 2/2] approve refuses malformed YAML"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve bad-yaml --json"; then
    fail "approve should fail on malformed YAML"
  fi
  assert_not_contains "$output" "Traceback" "approve should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'validation_failed'" "approve should surface a validation failure"
  assert_json "$output" "'invalid spec document' in ' '.join(data['error']['details'])" "approve details should explain the malformed spec"
}

run_runtime_phase() {
  local repo
  repo="$(new_repo)"

  mkdir -p "$repo/.ai/specs/approved"
  write_invalid_spec "$repo/.ai/specs/approved/bad-build.yaml" "bad-build" "approved"

  echo "[runtime 1/2] build fails as a structured malformed-spec error"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build bad-build --json"; then
    fail "build should fail on malformed approved YAML"
  fi
  assert_not_contains "$output" "Traceback" "build should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'invalid_spec_document'" "build should surface invalid_spec_document"

  mkdir -p "$repo/.ai/specs/drafts"
  write_invalid_spec "$repo/.ai/specs/drafts/bad-harden.yaml" "bad-harden" "draft"

  echo "[runtime 2/2] harden fails as a structured malformed-spec error"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld harden bad-harden --json"; then
    fail "harden should fail on malformed draft YAML"
  fi
  assert_not_contains "$output" "Traceback" "harden should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'invalid_spec_document'" "harden should surface invalid_spec_document"
}

case "$phase" in
  gate)
    run_gate_phase
    ;;
  runtime)
    run_runtime_phase
    ;;
  all)
    run_gate_phase
    run_runtime_phase
    ;;
  *)
    fail "unknown phase '$phase'"
    ;;
esac

echo "PASS: invalid yaml spec smoke"
