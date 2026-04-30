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
  repo="$(mktemp -d /tmp/scafld-invalid-markdown-smoke.XXXXXX)"
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
---
spec_version: '2.0'
task_id: $task_id
created: '2026-04-25T00:00:00Z'
updated: '2026-04-25T00:00:00Z'
status: $status
harden_status: in_progress
size: small
risk_level: low
---

# Broken spec

## Summary

Invalid Markdown fixture with an unclosed code fence.

## Acceptance

\`\`\`yaml
validation_profile: standard
definition_of_done:
  - id: dod1
    description: unclosed fence

## Phase 1: Broken phase

Goal: Exercise malformed Markdown handling.

Status: pending
Dependencies: none

Changes:
- \`foo.txt\` - fixture

Acceptance:
- [ ] \`ac1_1\` custom - Should not run.
  - Command: \`true\`
  - Expected kind: \`exit_code_zero\`

## Planning Log

- 2026-04-25T00:00:00Z - user - invalid Markdown fixture
EOF
}

run_gate_phase() {
  local repo
  repo="$(new_repo)"
  (cd "$repo" && scafld_cmd plan bad-markdown -t "Bad markdown" -s small -r low --json >/dev/null)
  write_invalid_spec "$repo/.scafld/specs/drafts/bad-markdown.md" "bad-markdown" "draft"

  echo "[gate 1/2] validate rejects malformed Markdown"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate bad-markdown --json"; then
    fail "validate should fail on malformed Markdown"
  fi
  assert_not_contains "$output" "Traceback" "validate should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'invalid_spec_document'" "validate should return a structured malformed-spec failure"
  assert_json "$output" "'unclosed Markdown code fence' in ' '.join(data['error']['details'])" "validate details should explain the malformed spec"

  echo "[gate 2/2] approve refuses malformed Markdown"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld approve bad-markdown --json"; then
    fail "approve should fail on malformed Markdown"
  fi
  assert_not_contains "$output" "Traceback" "approve should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'validation_failed'" "approve should surface a validation failure"
  assert_json "$output" "'unclosed Markdown code fence' in ' '.join(data['error']['details'])" "approve details should explain the malformed spec"
}

run_runtime_phase() {
  local repo
  repo="$(new_repo)"

  mkdir -p "$repo/.scafld/specs/approved"
  write_invalid_spec "$repo/.scafld/specs/approved/bad-build.md" "bad-build" "approved"

  echo "[runtime 1/2] build fails as a structured malformed-spec error"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build bad-build --json"; then
    fail "build should fail on malformed approved Markdown"
  fi
  assert_not_contains "$output" "Traceback" "build should fail without a traceback"
  assert_json "$output" "data['ok'] is False and data['error']['code'] == 'invalid_spec_document'" "build should surface invalid_spec_document"

  mkdir -p "$repo/.scafld/specs/drafts"
  write_invalid_spec "$repo/.scafld/specs/drafts/bad-harden.md" "bad-harden" "draft"

  echo "[runtime 2/2] harden fails as a structured malformed-spec error"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld harden bad-harden --json"; then
    fail "harden should fail on malformed draft Markdown"
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

echo "PASS: invalid markdown spec smoke"
