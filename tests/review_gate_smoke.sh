#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
SELF="$SCRIPT_DIR/$(basename "${BASH_SOURCE[0]}")"
TMP_DIRS=()

cleanup() {
  if [ "${#TMP_DIRS[@]}" -gt 0 ]; then
    rm -rf "${TMP_DIRS[@]}"
  fi
}
trap cleanup EXIT

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

capture() {
  local __var="$1"
  shift
  local _captured
  set +e
  _captured="$("$@" 2>&1)"
  local status=$?
  set -e
  printf -v "$__var" '%s' "$_captured"
  return "$status"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" != *"$needle"* ]]; then
    fail "$message"
  fi
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  if [[ "$haystack" == *"$needle"* ]]; then
    fail "$message"
  fi
}

trellis_cmd() {
  PATH="$CLI_ROOT:$PATH" trellis "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/trellis-review-smoke.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    trellis_cmd init >/dev/null
    printf 'base\n' > app.txt
    git add .
    git commit -m "init" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

write_active_spec() {
  local repo="$1"
  local task_id="$2"
  local command="$3"
  local expected="$4"
  local result_value="${5:-}"
  local result_block=""
  if [ -n "$result_value" ]; then
    result_block="        result: \"$result_value\""
  fi

  cat > "$repo/.ai/specs/active/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "in_progress"

task:
  title: "Smoke $task_id"
  summary: "Smoke fixture for review gate enforcement"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Smoke"
    objective: "Exercise the review gate"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "Smoke change"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "app.txt reflects the changed content"
        command: "$command"
        expected: "$expected"
${result_block}

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"
EOF
}

write_changed_file() {
  local repo="$1"
  printf 'changed\n' > "$repo/app.txt"
}

pass_label() {
  case "$1" in
    pass) printf 'PASS' ;;
    fail) printf 'FAIL' ;;
    not_run) printf 'NOT RUN' ;;
    *) printf '%s' "$1" ;;
  esac
}

write_review_v2() {
  local repo="$1"
  local task_id="$2"
  local verdict="$3"
  local reviewer_mode="$4"
  local round_status="$5"
  local spec_pass="$6"
  local scope_pass="$7"
  local blocking_body="$8"
  local non_blocking_body="$9"
  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/$task_id.md" <<EOF
# Review: $task_id

## Spec
Smoke $task_id
Smoke fixture for review gate enforcement

## Files Changed
- app.txt

---

## Review 1 — 2026-03-26T00:00:00Z

### Metadata
\`\`\`json
{
  "schema_version": 2,
  "round_status": "$round_status",
  "reviewer_mode": "$reviewer_mode",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-03-26T00:00:00Z",
  "override_reason": null,
  "automated_passes": {
    "spec_compliance": "$spec_pass",
    "scope_drift": "$scope_pass"
  }
}
\`\`\`

### Automated Passes
- spec_compliance: $(pass_label "$spec_pass")
- scope_drift: $(pass_label "$scope_pass")

### Blocking
$blocking_body

### Non-blocking
$non_blocking_body

### Verdict
$verdict
EOF
}

write_review_without_metadata() {
  local repo="$1"
  local task_id="$2"
  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/$task_id.md" <<EOF
# Review: $task_id

## Spec
Smoke $task_id

## Files Changed
- app.txt

---

## Review 1 — 2026-03-26T00:00:00Z

### Automated Passes
- spec_compliance: PASS
- scope_drift: PASS

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF
}

archive_spec_path() {
  local repo="$1"
  local task_id="$2"
  find "$repo/.ai/specs/archive" -name "$task_id.yaml" -print | head -n 1
}

case_smoke_bootstrap() {
  local repo task_id output
  repo="$(new_repo)"
  task_id="smoke-bootstrap"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" trellis status '$task_id'"
  assert_contains "$output" "Smoke $task_id" "smoke bootstrap should expose a valid fixture repo"
}

case_human_override() {
  local repo task_id output archive_path review_text spec_text
  repo="$(new_repo)"
  task_id="human-override"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "false" "exit code 0"

  capture output bash -lc "PATH='$CLI_ROOT':\"\$PATH\" trellis complete --help"
  assert_contains "$output" "--human-reviewed" "complete help should expose --human-reviewed"
  assert_not_contains "$output" "--force" "complete help should no longer expose --force"

  if capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | PATH='$CLI_ROOT':\"\$PATH\" trellis complete '$task_id' --human-reviewed --reason 'manual audit'"; then
    fail "piped override should be rejected"
  fi
  assert_contains "$output" "interactive terminal" "piped override should mention the TTY requirement"

  capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" trellis complete '\''$task_id'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "interactive override should archive the spec"
  spec_text="$(cat "$archive_path")"
  review_text="$(cat "$repo/.ai/reviews/$task_id.md")"
  assert_contains "$spec_text" 'override_applied: true' "archived spec should record override_applied"
  assert_contains "$spec_text" 'override_reason: "manual audit"' "archived spec should record override_reason"
  assert_contains "$spec_text" 'result: "fail"' "archived spec should record the real failing automated pass state"
  assert_contains "$spec_text" 'result: "pass"' "archived spec should record the real passing automated pass state"
  assert_contains "$review_text" '"reviewer_mode": "human_override"' "review file should record human_override provenance"
}

case_duplicate_task_id() {
  local repo task_id output
  repo="$(new_repo)"
  task_id="duplicate-task-id"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"
  mkdir -p "$repo/.ai/specs/archive/2026-03"
  cp "$repo/.ai/specs/active/$task_id.yaml" "$repo/.ai/specs/archive/2026-03/$task_id.yaml"

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" trellis status '$task_id'"; then
    fail "duplicate task ids should be rejected"
  fi
  assert_contains "$output" "ambiguous task-id" "duplicate task ids should report ambiguity"
}

case_failed_review_round() {
  local repo task_id output review_count
  repo="$(new_repo)"
  task_id="failed-review-round"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "false" "exit code 0"
  write_review_v2 "$repo" "$task_id" "pass" "fresh_agent" "completed" "pass" "pass" "None." "None."

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" trellis review '$task_id'"; then
    fail "review should fail when automated checks fail"
  fi
  review_count="$(grep -c '^## Review ' "$repo/.ai/reviews/$task_id.md")"
  [ "$review_count" = "1" ] || fail "failed automated review should not append a new round"
}

case_malformed_review() {
  local repo task_id output
  repo="$(new_repo)"
  task_id="malformed-review"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"
  write_review_without_metadata "$repo" "$task_id"

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" trellis complete '$task_id'"; then
    fail "metadata-free review should be rejected"
  fi
  assert_contains "$output" "malformed or incomplete" "complete should reject malformed review rounds"
}

case_provenance_and_results() {
  local repo task_id output archive_path spec_text review_text
  repo="$(new_repo)"
  task_id="provenance-and-results"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0"
  write_review_v2 "$repo" "$task_id" "fail" "executor" "completed" "fail" "pass" '- **high** `app.txt:1` — blocker' 'None.'

  capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" trellis complete '\''$task_id'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "override should archive the spec"
  spec_text="$(cat "$archive_path")"
  review_text="$(cat "$repo/.ai/reviews/$task_id.md")"
  assert_contains "$spec_text" 'verdict: "fail"' "archived spec should preserve the underlying fail verdict"
  assert_contains "$spec_text" 'reviewer_mode: "human_override"' "archived spec should record reviewer_mode"
  assert_contains "$spec_text" 'result: "fail"' "archived spec should keep the real spec_compliance result"
  assert_contains "$spec_text" 'result: "pass"' "archived spec should keep the real scope_drift result"
  assert_contains "$review_text" '"override_reason": "manual audit"' "override round should record the provided reason"
}

case_non_mutating_review() {
  local repo task_id before after
  repo="$(new_repo)"
  task_id="non-mutating-review"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "fail"
  before="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" trellis review '$task_id'" >/dev/null
  after="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  if [ "$before" != "$after" ]; then
    fail "trellis review should not mutate existing execution evidence"
  fi
}

case_all() {
  case_smoke_bootstrap
  case_human_override
  case_duplicate_task_id
  case_failed_review_round
  case_malformed_review
  case_provenance_and_results
  case_non_mutating_review
}

main() {
  local action="${1:-all}"
  case "$action" in
    smoke-bootstrap) case_smoke_bootstrap ;;
    human-override) case_human_override ;;
    duplicate-task-id) case_duplicate_task_id ;;
    failed-review-round) case_failed_review_round ;;
    malformed-review) case_malformed_review ;;
    provenance-and-results) case_provenance_and_results ;;
    non-mutating-review) case_non_mutating_review ;;
    all) case_all ;;
    *)
      fail "unknown case: $action"
      ;;
  esac
}

main "$@"
