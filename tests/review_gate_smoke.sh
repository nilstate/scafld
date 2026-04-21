#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI_ROOT="${CLI_ROOT:-$REPO_ROOT/cli}"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

assert_file_order() {
  local file="$1"
  local first="$2"
  local second="$3"
  local message="$4"
  local first_line second_line
  if command -v rg >/dev/null 2>&1; then
    first_line="$(rg -n -F -- "$first" "$file" | head -n 1 | cut -d: -f1 || true)"
    second_line="$(rg -n -F -- "$second" "$file" | head -n 1 | cut -d: -f1 || true)"
  else
    first_line="$(grep -n -F -- "$first" "$file" | head -n 1 | cut -d: -f1 || true)"
    second_line="$(grep -n -F -- "$second" "$file" | head -n 1 | cut -d: -f1 || true)"
  fi
  [ -n "$first_line" ] || fail "$message (missing '$first')"
  [ -n "$second_line" ] || fail "$message (missing '$second')"
  [ "$first_line" -lt "$second_line" ] || fail "$message"
}

scafld_cmd() {
  PATH="$CLI_ROOT:$PATH" scafld "$@"
}

new_repo() {
  local repo
  repo="$(mktemp -d /tmp/scafld-review-smoke.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
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

write_local_order_override() {
  local repo="$1"
  cat > "$repo/.ai/config.local.yaml" <<'EOF'
review:
  adversarial_passes:
    convention_check:
      order: 30
      title: "Convention Check"
      description: "Check changed code against the documented rules"
    regression_hunt:
      order: 40
      title: "Regression Hunt"
      description: "Trace callers and downstream consumers for regressions"
    dark_patterns:
      order: 50
      title: "Dark Patterns"
      description: "Hunt for subtle bugs and hardcoded shortcuts"
EOF
}

write_local_title_override() {
  local repo="$1"
  cat > "$repo/.ai/config.local.yaml" <<'EOF'
review:
  adversarial_passes:
    regression_hunt:
      order: 30
      title: "Regression Hunt"
      description: "Trace callers and downstream consumers for regressions"
    convention_check:
      order: 40
      title: "Convention Check"
      description: "Check changed code against the documented rules"
    dark_patterns:
      order: 50
      title: "Defect Sweep"
      description: "Hunt for subtle bugs and hardcoded shortcuts"
EOF
}

pass_label() {
  case "$1" in
    pass) printf 'PASS' ;;
    fail) printf 'FAIL' ;;
    pass_with_issues) printf 'PASS WITH ISSUES' ;;
    not_run) printf 'NOT RUN' ;;
    *) printf '%s' "$1" ;;
  esac
}

inject_review_git_state() {
  local repo="$1"
  local task_id="$2"
  REVIEW_REPO="$repo" REVIEW_TASK_ID="$task_id" CLI_SCRIPT="$REPO_ROOT/cli/scafld" python3 - <<'PY'
import json
import os
import pathlib
import re
import runpy

repo = pathlib.Path(os.environ["REVIEW_REPO"])
task_id = os.environ["REVIEW_TASK_ID"]
namespace = runpy.run_path(os.environ["CLI_SCRIPT"])
state, error = namespace["capture_review_git_state"](repo, f".ai/reviews/{task_id}.md")
if error:
    raise SystemExit(error)

review_path = repo / ".ai" / "reviews" / f"{task_id}.md"
text = review_path.read_text()
matches = list(re.finditer(r"```json\s*\n(.*?)\n```", text, re.DOTALL))
if not matches:
    raise SystemExit("review metadata JSON block not found")

metadata_match = matches[-1]
metadata = json.loads(metadata_match.group(1))
metadata.update(state)
review_path.write_text(text[:metadata_match.start(1)] + json.dumps(metadata, indent=2) + text[metadata_match.end(1):])
PY
}

write_review_v3() {
  local repo="$1"
  local task_id="$2"
  local verdict="$3"
  local reviewer_mode="$4"
  local round_status="$5"
  local spec_pass="$6"
  local scope_pass="$7"
  local regression_pass="$8"
  local convention_pass="$9"
  local dark_pass="${10}"
  local regression_body="${11}"
  local convention_body="${12}"
  local dark_body="${13}"
  local blocking_body="${14}"
  local non_blocking_body="${15}"

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
  "schema_version": 3,
  "round_status": "$round_status",
  "reviewer_mode": "$reviewer_mode",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-03-26T00:00:00Z",
  "override_reason": null,
  "pass_results": {
    "spec_compliance": "$spec_pass",
    "scope_drift": "$scope_pass",
    "regression_hunt": "$regression_pass",
    "convention_check": "$convention_pass",
    "dark_patterns": "$dark_pass"
  }
}
\`\`\`

### Pass Results
- spec_compliance: $(pass_label "$spec_pass")
- scope_drift: $(pass_label "$scope_pass")
- regression_hunt: $(pass_label "$regression_pass")
- convention_check: $(pass_label "$convention_pass")
- dark_patterns: $(pass_label "$dark_pass")

### Regression Hunt
$regression_body

### Convention Check
$convention_body

### Dark Patterns
$dark_body

### Blocking
$blocking_body

### Non-blocking
$non_blocking_body

### Verdict
$verdict
EOF
  inject_review_git_state "$repo" "$task_id"
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

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of app.txt.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked hardcodes and null handling.

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
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status '$task_id'"
  assert_contains "$output" "Smoke $task_id" "smoke bootstrap should expose a valid fixture repo"
}

case_review_pass_topology() {
  local repo task_id output review_file review_text
  repo="$(new_repo)"
  task_id="review-pass-topology"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0"
  write_local_order_override "$repo"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$task_id'"
  review_file="$repo/.ai/reviews/$task_id.md"
  review_text="$(cat "$review_file")"

  assert_contains "$review_text" '"schema_version": 3' "review metadata should use schema_version 3"
  assert_contains "$review_text" '"round_status": "in_progress"' "review metadata should start in_progress"
  assert_contains "$review_text" '"reviewed_head": "' "review metadata should record the reviewed commit"
  assert_contains "$review_text" '"reviewed_dirty": true' "review metadata should record dirty workspace state"
  assert_contains "$review_text" '"reviewed_diff": "' "review metadata should record the reviewed workspace fingerprint"
  assert_contains "$review_text" '"pass_results": {' "review metadata should include pass_results"
  assert_contains "$review_text" '"convention_check": "not_run"' "adversarial pass results should be scaffolded"
  assert_file_order "$review_file" "- convention_check: NOT RUN" "- regression_hunt: NOT RUN" "pass results should follow configured order"
  assert_file_order "$review_file" "### Convention Check" "### Regression Hunt" "adversarial section order should follow configured order"
}

case_review_scaffold_topology() {
  local repo task_id output review_text
  repo="$(new_repo)"
  task_id="review-scaffold-topology"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0"
  write_local_title_override "$repo"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$task_id'"
  review_text="$(cat "$repo/.ai/reviews/$task_id.md")"
  assert_contains "$review_text" '### Defect Sweep' "review scaffold should use configured adversarial titles"
  assert_not_contains "$review_text" '### Dark Patterns' "review scaffold should not hardcode default section titles"
  assert_contains "$output" '### Defect Sweep' "review prompt should reference the configured section title"
}

case_review_complete_topology() {
  local repo task_id output archive_path spec_text
  repo="$(new_repo)"
  task_id="review-complete-topology"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"
  write_local_title_override "$repo"

  mkdir -p "$repo/.ai/reviews"
  cat > "$repo/.ai/reviews/$task_id.md" <<'EOF'
# Review: review-complete-topology

## Spec
Smoke review-complete-topology
Smoke fixture for review gate enforcement

## Files Changed
- app.txt

---

## Review 1 — 2026-03-26T00:00:00Z

### Metadata
```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-03-26T00:00:00Z",
  "override_reason": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass_with_issues"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS WITH ISSUES

### Regression Hunt
No issues found — checked callers of app.txt.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
- **low** `app.txt:1` — placeholder title is wrong for this repo.

### Blocking
None.

### Non-blocking
- **low** `app.txt:1` — placeholder title is wrong for this repo.

### Verdict
pass_with_issues
EOF
  inject_review_git_state "$repo" "$task_id"

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id'"; then
    fail "complete should reject review files that miss configured section titles"
  fi
  assert_contains "$output" "configured review sections incomplete — missing: Defect Sweep" "complete should validate configured section headings"

  cat > "$repo/.ai/reviews/$task_id.md" <<'EOF'
# Review: review-complete-topology

## Spec
Smoke review-complete-topology
Smoke fixture for review gate enforcement

## Files Changed
- app.txt

---

## Review 1 — 2026-03-26T00:00:00Z

### Metadata
```json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "fresh_agent",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-03-26T00:00:00Z",
  "override_reason": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass_with_issues"
  }
}
```

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS WITH ISSUES

### Regression Hunt
No issues found — checked callers of app.txt.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Defect Sweep
- **low** `app.txt:1` — placeholder title is wrong for this repo.

### Blocking
None.

### Non-blocking
- **low** `app.txt:1` — placeholder title is wrong for this repo.

### Verdict
pass_with_issues
EOF
  inject_review_git_state "$repo" "$task_id"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id'"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "complete should archive the spec after a valid v3 review"
  spec_text="$(cat "$archive_path")"
  assert_contains "$spec_text" 'verdict: "pass_with_issues"' "archived spec should preserve the final verdict"
  assert_contains "$spec_text" '- id: regression_hunt' "archived spec should record the configured adversarial passes"
  assert_contains "$spec_text" '- id: convention_check' "archived spec should record the configured adversarial passes"
  assert_contains "$spec_text" '- id: dark_patterns' "archived spec should record the configured adversarial passes"
  assert_contains "$spec_text" 'result: "pass_with_issues"' "archived spec should preserve per-pass results"
}

case_review_git_binding() {
  local repo task_id output archive_path spec_text
  repo="$(new_repo)"
  task_id="review-git-binding"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"
  write_review_v3 \
    "$repo" "$task_id" "pass" "fresh_agent" "completed" \
    "pass" "pass" "pass" "pass" "pass" \
    "No issues found — checked callers of app.txt." \
    "No issues found — checked AGENTS.md and CONVENTIONS.md." \
    "No issues found — checked hardcodes and null handling." \
    "None." "None."

  printf 'changed again\n' > "$repo/app.txt"
  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id'"; then
    fail "complete should reject reviews after the workspace changes"
  fi
  assert_contains "$output" "current workspace no longer matches the reviewed git state" "complete should block when the reviewed diff no longer matches"

  capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" scafld complete '\''$task_id'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "override should archive a git-bound review"
  spec_text="$(cat "$archive_path")"
  assert_contains "$spec_text" 'reviewed_head: "' "archived spec should record the reviewed commit"
  assert_contains "$spec_text" 'reviewed_diff: "' "archived spec should record the reviewed workspace fingerprint"
}

case_human_override() {
  local repo task_id output archive_path review_text spec_text
  repo="$(new_repo)"
  task_id="human-override"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "false" "exit code 0"

  capture output bash -lc "PATH='$CLI_ROOT':\"\$PATH\" scafld complete --help"
  assert_contains "$output" "--human-reviewed" "complete help should expose --human-reviewed"
  assert_not_contains "$output" "--force" "complete help should no longer expose --force"

  if capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id' --human-reviewed --reason 'manual audit'"; then
    fail "piped override should be rejected"
  fi
  assert_contains "$output" "interactive terminal" "piped override should mention the TTY requirement"

  capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" scafld complete '\''$task_id'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "interactive override should archive the spec"
  spec_text="$(cat "$archive_path")"
  review_text="$(cat "$repo/.ai/reviews/$task_id.md")"

  assert_contains "$spec_text" 'override_applied: true' "archived spec should record override_applied"
  assert_contains "$spec_text" 'override_reason: "manual audit"' "archived spec should record override_reason"
  assert_contains "$spec_text" '- id: spec_compliance' "archived spec should record spec_compliance"
  assert_contains "$spec_text" '- id: scope_drift' "archived spec should record scope_drift"
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

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status '$task_id'"; then
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
  write_review_v3 \
    "$repo" "$task_id" "pass" "fresh_agent" "completed" \
    "pass" "pass" "pass" "pass" "pass" \
    "No issues found — checked callers of app.txt." \
    "No issues found — checked AGENTS.md and CONVENTIONS.md." \
    "No issues found — checked hardcodes and null handling." \
    "None." "None."

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$task_id'"; then
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

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id'"; then
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
  write_review_v3 \
    "$repo" "$task_id" "fail" "executor" "completed" \
    "fail" "pass" "fail" "pass" "pass" \
    '- **high** `app.txt:1` — caller contract broken.' \
    "No issues found — checked AGENTS.md and CONVENTIONS.md." \
    "No issues found — checked hardcodes and null handling." \
    '- **high** `app.txt:1` — blocker' \
    "None."

  capture output bash -lc "cd '$repo' && printf '%s\n' '$task_id' | script -qefc 'PATH='\''$CLI_ROOT'\'':\"\$PATH\" scafld complete '\''$task_id'\'' --human-reviewed --reason '\''manual audit'\''' /dev/null"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "override should archive the spec"
  spec_text="$(cat "$archive_path")"
  review_text="$(cat "$repo/.ai/reviews/$task_id.md")"

  assert_contains "$spec_text" 'verdict: "fail"' "archived spec should preserve the underlying fail verdict"
  assert_contains "$spec_text" 'reviewer_mode: "human_override"' "archived spec should record reviewer_mode"
  assert_contains "$spec_text" '- id: spec_compliance' "archived spec should preserve the configured pass list"
  assert_contains "$spec_text" '- id: regression_hunt' "archived spec should preserve the configured pass list"
  assert_contains "$spec_text" 'result: "fail"' "archived spec should keep failing pass results"
  assert_contains "$spec_text" 'result: "pass"' "archived spec should keep passing pass results"
  assert_contains "$review_text" '"override_reason": "manual audit"' "override round should record the provided reason"
}

case_non_mutating_review() {
  local repo task_id before after
  repo="$(new_repo)"
  task_id="non-mutating-review"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "fail"
  before="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$task_id'" >/dev/null
  after="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  if [ "$before" != "$after" ]; then
    fail "scafld review should not mutate existing execution evidence"
  fi
}

case_exec_resume_nested_results() {
  local repo task_id output spec_text
  repo="$(new_repo)"
  task_id="exec-resume-nested-results"
  write_changed_file "$repo"

  cat > "$repo/.ai/specs/active/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "in_progress"

task:
  title: "Resume nested results"
  summary: "Exercise nested result parsing and generic pass expectations"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Execution checks"
    objective: "Exercise resume parsing and generic pass expectations"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "Smoke change"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "Already passed criterion"
        command: "printf '1 example, 0 failures\\n'"
        expected: "All pass"
        result:
          status: "pass"
          timestamp: "2026-03-26T00:00:00Z"
          output: "1 example, 0 failures"
      - id: "ac1_2"
        type: "test"
        description: "Generic pass phrase still succeeds"
        command: "printf '2 examples, 0 failures\\n'"
        expected: "All pass"

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"
EOF

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec '$task_id' --resume"
  assert_contains "$output" "resume: skipping 1 already-passed criteria" "--resume should skip nested pass results"
  assert_contains "$output" "ac1_2" "exec should still run the pending criterion"
  assert_not_contains "$output" "ac1_1: Already passed criterion" "skipped criterion should not be re-executed"
  spec_text="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  assert_contains "$spec_text" 'result: "pass"' "executed criterion should record a passing result"
}

case_exec_timeout_override() {
  local repo task_id output spec_text
  repo="$(new_repo)"
  task_id="exec-timeout-override"
  write_changed_file "$repo"

  cat > "$repo/.ai/specs/active/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "in_progress"

task:
  title: "Timeout override"
  summary: "Exercise per-criterion timeout overrides during execution"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Execution timeout"
    objective: "Exercise per-criterion timeout overrides"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "Smoke change"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "Timeout override is enforced"
        command: "python3 -c \"import time; time.sleep(2)\""
        expected: "exit code 0"
        timeout_seconds: 1

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"
EOF

  if capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec '$task_id'"; then
    fail "timeout override should fail when the command exceeds timeout_seconds"
  fi
  assert_contains "$output" "TIMEOUT (1s)" "exec should report the configured timeout"
  spec_text="$(cat "$repo/.ai/specs/active/$task_id.yaml")"
  assert_contains "$spec_text" 'Command timed out after 1s' "spec should record the configured timeout in result_output"
}

case_complete_nested_exec_and_self_eval() {
  local repo task_id output archive_path spec_text
  repo="$(new_repo)"
  task_id="complete-nested-exec-and-self-eval"
  write_changed_file "$repo"

  cat > "$repo/.ai/specs/active/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "in_progress"

task:
  title: "Nested exec and self-eval"
  summary: "Ensure scafld complete recognizes nested acceptance results and self_eval totals"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Nested result shape"
    objective: "Exercise nested acceptance result parsing"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "Smoke change"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "Nested result should count as executed"
        command: "printf '1 example, 0 failures\\n'"
        expected: "0 failures"
        result:
          status: "pass"
          timestamp: "2026-03-26T00:00:00Z"
          output: "1 example, 0 failures"
    status: "completed"

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"

self_eval:
  completeness: 3
  architecture_fidelity: 3
  spec_alignment: 1
  validation_depth: 1
  total: 8.8
  notes: "Nested score fixture"
  second_pass_performed: false

deviations: []
EOF

  write_review_v3 "$repo" "$task_id" "pass" "executor" "completed" "pass" "pass" "pass" "pass" "pass" \
    "No issues found — checked callers of app.txt." \
    "No issues found — checked AGENTS.md and CONVENTIONS.md." \
    "No issues found — checked hardcodes and null handling." \
    "None." \
    "None."

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id'"
  assert_not_contains "$output" "no exec results recorded" "complete should recognize nested acceptance results recorded by scafld exec"
  assert_not_contains "$output" "no self-eval score found in spec" "complete should recognize nested self_eval totals"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "complete should archive the nested exec fixture"
  spec_text="$(cat "$archive_path")"
  assert_contains "$spec_text" 'status: "completed"' "archived spec should remain completed"
}

case_report_nested_exec_and_self_eval() {
  local repo task_id output
  repo="$(new_repo)"
  task_id="report-nested-exec-and-self-eval"

  mkdir -p "$repo/.ai/specs/archive/2026-03"
  cat > "$repo/.ai/specs/archive/2026-03/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "completed"

task:
  title: "Report nested parsing"
  summary: "Ensure scafld report counts nested execution results and decimal self-eval totals"
  size: "small"
  risk_level: "low"

phases:
  - id: "phase1"
    name: "Nested result shape"
    objective: "Exercise nested acceptance result parsing in report"
    changes:
      - file: "app.txt"
        action: "update"
        lines: "1"
        content_spec: "Smoke change"
    acceptance_criteria:
      - id: "ac1_1"
        type: "test"
        description: "Nested pass result should count"
        command: "printf '1 example, 0 failures\\n'"
        expected: "0 failures"
        result:
          status: "pass"
          timestamp: "2026-03-26T00:00:00Z"
          output: "1 example, 0 failures"
    status: "completed"

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"

self_eval:
  completeness: 3
  architecture_fidelity: 3
  spec_alignment: 1
  validation_depth: 1
  total: 8.8
  notes: "Nested score fixture"
  second_pass_performed: false

deviations: []
EOF

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report"
  assert_contains "$output" "avg: 8.8/10  (1 scored)" "report should read decimal self-eval totals from the nested block"
  assert_contains "$output" "1 passed / 0 failed  (100% pass rate)" "report should count nested acceptance results"
}

case_status_phase_count_ignores_top_level_status() {
  local repo task_id output
  repo="$(new_repo)"
  task_id="status-phase-count"

  mkdir -p "$repo/.ai/specs/archive/2026-03"
  cat > "$repo/.ai/specs/archive/2026-03/$task_id.yaml" <<EOF
spec_version: "1.1"
task_id: "$task_id"
created: "2026-03-26T00:00:00Z"
updated: "2026-03-26T00:00:00Z"
status: "completed"

task:
  title: "Status phase counts"
  summary: "Ensure scafld status only counts statuses from the phases section"
  size: "small"
  risk_level: "low"

task_notes:
  status: "completed"

phases:
  - id: "phase1"
    name: "One"
    objective: "One"
    changes:
      - file: "app.txt"
        action: "update"
        content_spec: "One"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "One"
        result:
          status: "pass"
    status: "completed"
  - id: "phase2"
    name: "Two"
    objective: "Two"
    changes:
      - file: "app.txt"
        action: "update"
        content_spec: "Two"
    acceptance_criteria:
      - id: "ac2_1"
        type: "custom"
        description: "Two"
        result:
          status: "pass"
    status: "completed"
  - id: "phase3"
    name: "Three"
    objective: "Three"
    changes:
      - file: "app.txt"
        action: "update"
        content_spec: "Three"
    acceptance_criteria:
      - id: "ac3_1"
        type: "custom"
        description: "Three"
        result:
          status: "pass"
    status: "completed"

planning_log:
  - timestamp: "2026-03-26T00:00:00Z"
    actor: "user"
    summary: "Bootstrap smoke fixture"
EOF

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status '$task_id'"
  assert_contains "$output" "phases: 3 done  (3 total)" "status should count only phase statuses and not subtract the top-level spec status"
  assert_not_contains "$output" "1 pending" "status should not invent a pending phase when all phase statuses are completed"
}

case_json_outputs() {
  local repo task_id output archive_path
  repo="$(new_repo)"
  task_id="json-outputs"
  write_changed_file "$repo"
  write_active_spec "$repo" "$task_id" "grep -q '^changed$' app.txt" "exit code 0" "pass"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld status '$task_id' --json"
  assert_json "$output" "data['command'] == 'status' and data['task_id'] == 'json-outputs' and data['state']['status'] == 'in_progress'" "status --json should emit task identity and status"
  assert_json "$output" "data['result']['phase_statuses'][0]['id'] == 'phase1'" "status --json should emit phase statuses"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate '$task_id' --json"
  assert_json "$output" "data['command'] == 'validate' and data['result']['valid'] is True and data['state']['schema_version'] == '1.1'" "validate --json should emit valid=true"

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review '$task_id' --json"
  assert_json "$output" "data['command'] == 'review' and data['ok'] is True and data['state']['review_round'] == 1" "review --json should open a structured review round"
  assert_json "$output" "'ADVERSARIAL REVIEW' in data['result']['review_prompt'] and data['result']['review_file'].endswith('json-outputs.md')" "review --json should include prompt and review file"
  assert_json "$output" "'Regression Hunt' in data['result']['required_sections']" "review --json should list required adversarial sections"

  write_review_v3 \
    "$repo" "$task_id" "pass" "executor" "completed" \
    "pass" "pass" "pass" "pass" "pass" \
    "No issues found — checked callers of app.txt." \
    "No issues found — checked AGENTS.md and CONVENTIONS.md." \
    "No issues found — checked hardcodes and null handling." \
    "None." "None."

  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete '$task_id' --json"
  assert_json "$output" "data['command'] == 'complete' and data['ok'] is True and data['state']['status'] == 'completed' and data['state']['review_verdict'] == 'pass'" "complete --json should emit completion state and verdict"
  assert_json "$output" "data['result']['blocking_count'] == 0 and data['result']['archive_path'].endswith('json-outputs.yaml')" "complete --json should emit gate counts and archive path"
  archive_path="$(archive_spec_path "$repo" "$task_id")"
  [ -n "$archive_path" ] || fail "complete --json should archive the spec"
}

case_all() {
  case_smoke_bootstrap
  case_review_pass_topology
  case_review_scaffold_topology
  case_review_complete_topology
  case_review_git_binding
  case_human_override
  case_duplicate_task_id
  case_failed_review_round
  case_malformed_review
  case_provenance_and_results
  case_non_mutating_review
  case_exec_resume_nested_results
  case_exec_timeout_override
  case_complete_nested_exec_and_self_eval
  case_report_nested_exec_and_self_eval
  case_status_phase_count_ignores_top_level_status
  case_json_outputs
}

main() {
  local action="${1:-all}"
  case "$action" in
    smoke-bootstrap) case_smoke_bootstrap ;;
    review-pass-topology) case_review_pass_topology ;;
    review-scaffold-topology) case_review_scaffold_topology ;;
    review-complete-topology) case_review_complete_topology ;;
    review-git-binding) case_review_git_binding ;;
    human-override) case_human_override ;;
    duplicate-task-id) case_duplicate_task_id ;;
    failed-review-round) case_failed_review_round ;;
    malformed-review) case_malformed_review ;;
    provenance-and-results) case_provenance_and_results ;;
    non-mutating-review) case_non_mutating_review ;;
    exec-resume-nested-results) case_exec_resume_nested_results ;;
    exec-timeout-override) case_exec_timeout_override ;;
    complete-nested-exec-and-self-eval) case_complete_nested_exec_and_self_eval ;;
    report-nested-exec-and-self-eval) case_report_nested_exec_and_self_eval ;;
    status-phase-count-ignores-top-level-status) case_status_phase_count_ignores_top_level_status ;;
    json-outputs) case_json_outputs ;;
    all) case_all ;;
    *)
      fail "unknown case: $action"
      ;;
  esac
}

main "$@"
