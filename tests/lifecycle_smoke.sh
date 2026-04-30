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
  repo="$(mktemp -d /tmp/scafld-lifecycle-smoke.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
  )
  printf '%s\n' "$repo"
}

inject_review_git_state() {
  local repo="$1"
  local task_id="$2"
  REVIEW_REPO="$repo" REVIEW_TASK_ID="$task_id" REPO_ROOT="$REPO_ROOT" python3 - <<'PY'
import json
import os
import pathlib
import re
import sys

repo = pathlib.Path(os.environ["REVIEW_REPO"])
task_id = os.environ["REVIEW_TASK_ID"]
sys.path.insert(0, os.environ["REPO_ROOT"])
from scafld.git_state import capture_review_git_state
from scafld.review_workflow import review_binding_excluded_rels

# Match the runtime exclusion list so the captured baseline lines up
# with what scafld complete recomputes.
excluded = review_binding_excluded_rels(task_id, f".scafld/reviews/{task_id}.md")
state, error = capture_review_git_state(repo, excluded)
if error:
    raise SystemExit(error)

review_path = repo / ".scafld" / "reviews" / f"{task_id}.md"
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

write_review_pass() {
  local repo="$1"
  local task_id="$2"
  local changed_file="$3"

  mkdir -p "$repo/.scafld/reviews"
  cat > "$repo/.scafld/reviews/$task_id.md" <<EOF
# Review: $task_id

## Spec
Lifecycle smoke $task_id
Lifecycle smoke fixture

## Files Changed
- $changed_file

---

## Review 1 — 2026-04-21T00:00:00Z

### Metadata
\`\`\`json
{
  "schema_version": 3,
  "round_status": "completed",
  "reviewer_mode": "challenger",
  "reviewer_session": "sess-1",
  "reviewed_at": "2026-04-21T00:00:00Z",
  "override_reason": null,
  "pass_results": {
    "spec_compliance": "pass",
    "scope_drift": "pass",
    "regression_hunt": "pass",
    "convention_check": "pass",
    "dark_patterns": "pass"
  }
}
\`\`\`

### Pass Results
- spec_compliance: PASS
- scope_drift: PASS
- regression_hunt: PASS
- convention_check: PASS
- dark_patterns: PASS

### Regression Hunt
No issues found — checked callers of $changed_file.

### Convention Check
No issues found — checked AGENTS.md and CONVENTIONS.md.

### Dark Patterns
No issues found — checked $changed_file content boundary and lifecycle completion path.

### Blocking
None.

### Non-blocking
None.

### Verdict
pass
EOF
  inject_review_git_state "$repo" "$task_id"
}

write_spec_fixture() {
  local path="$1"
  local task_id="$2"
  local status="$3"
  local updated="$4"
  local file_name="$5"
  local include_result="${6:-no}"
  local phase_status="${7:-pending}"

  if [ "$include_result" = "yes" ]; then
    env SCAFLD_SPEC_CREATED="$updated" \
      SCAFLD_SPEC_UPDATED="$updated" \
      SCAFLD_SPEC_PHASE_STATUS="$phase_status" \
      SCAFLD_SPEC_CRITERION_RESULT=pass \
      PYTHONPATH="$REPO_ROOT" \
      python3 "$SMOKE_LIB_DIR/spec_fixture.py" \
      "$path" "$task_id" "$status" "Fixture $task_id" "$file_name" "grep -q '^ok$' $file_name"
    return
  fi

  env SCAFLD_SPEC_CREATED="$updated" \
    SCAFLD_SPEC_UPDATED="$updated" \
    SCAFLD_SPEC_PHASE_STATUS="$phase_status" \
    PYTHONPATH="$REPO_ROOT" \
    python3 "$SMOKE_LIB_DIR/spec_fixture.py" \
    "$path" "$task_id" "$status" "Fixture $task_id" "$file_name" "grep -q '^ok$' $file_name"
}

main() {
  local repo draft_path active_path archive_path output report_output
  repo="$(new_repo)"

  echo "[1/6] init a clean workspace and baseline commit"
  (
    cd "$repo"
    scafld_cmd init >/dev/null
    printf 'base\n' > demo.txt
    git add .
    git commit -m "bootstrap" >/dev/null 2>&1
  )

  echo "[2/6] create a real draft through plan, make it valid, and validate it"
  (
    cd "$repo"
    scafld_cmd plan demo-task -t "Lifecycle smoke" -s small -r low >/dev/null
  )
  draft_path="$repo/.scafld/specs/drafts/demo-task.md"
  SCAFLD_SPEC_CREATED="2026-04-21T00:00:00Z" \
    write_markdown_spec "$draft_path" "demo-task" "draft" "Lifecycle smoke" "demo.txt" "grep -q '^done$' demo.txt"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate demo-task"
  assert_contains "$output" "PASS:" "validate should accept the lifecycle spec"

  echo "[3/6] approve and build the spec"
  (
    cd "$repo"
    scafld_cmd approve demo-task >/dev/null
    printf 'done\n' > demo.txt
  )
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld build demo-task"
  assert_contains "$output" "1 passed" "build should record the passing acceptance criterion"

  echo "[4/6] run review and complete the spec"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review demo-task --runner manual"
  assert_contains "$output" "ADVERSARIAL REVIEW" "review should emit the handoff prompt"
  write_review_pass "$repo" "demo-task" "demo.txt"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete demo-task"
  assert_contains "$output" "review" "complete should print the final review verdict"
  archive_path="$(find "$repo/.scafld/specs/archive" -name demo-task.md -print | head -n 1)"
  [ -n "$archive_path" ] || fail "complete should archive the lifecycle spec"
  run_archive="$(find "$repo/.scafld/runs/archive" -type d -name demo-task -print | head -n 1)"
  [ -n "$run_archive" ] || fail "complete should archive the run directory"
  capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld handoff demo-task --json"
  assert_json "$output" "data['state']['role'] == 'challenger'" "default handoff after completion should be the challenger role"
  assert_json "$output" "data['state']['gate'] == 'review'" "default handoff after completion should be the review gate"
  assert_json "$output" "'/archive/' in data['result']['handoff_file']" "completed handoff should come from the archived run dir"

  echo "[5/6] create triage fixtures for report output"
  mkdir -p "$repo/.scafld/specs/drafts" "$repo/.scafld/specs/approved" "$repo/.scafld/specs/active"
  write_spec_fixture "$repo/.scafld/specs/drafts/stale-draft.md" "stale-draft" "draft" "2025-01-01T00:00:00Z" "stale.txt"
  write_spec_fixture "$repo/.scafld/specs/approved/waiting-spec.md" "waiting-spec" "approved" "2026-04-01T00:00:00Z" "waiting.txt"
  write_spec_fixture "$repo/.scafld/specs/active/no-exec.md" "no-exec" "in_progress" "2026-04-01T00:00:00Z" "no-exec.txt"
  write_spec_fixture "$repo/.scafld/specs/active/stale-active.md" "stale-active" "in_progress" "2026-04-01T00:00:00Z" "stale-active.txt" "yes" "completed"
  write_spec_fixture "$repo/.scafld/specs/active/review-drift.md" "review-drift" "in_progress" "2026-04-01T00:00:00Z" "drift.txt" "yes"
  write_spec_fixture "$repo/.scafld/specs/archive/2026-04/superseded-old.md" "superseded-old" "cancelled" "2026-04-02T00:00:00Z" "old.txt" "yes" "completed"
  PYTHONPATH="$REPO_ROOT" python3 - "$repo/.scafld/specs/archive/2026-04/superseded-old.md" <<'PY'
import sys
from pathlib import Path
from scafld.spec_markdown import parse_spec_markdown, update_spec_markdown
path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
data = parse_spec_markdown(text)
data["origin"] = {"origin": None, "sync": None, "supersession": {"superseded_by": "demo-task", "superseded_at": "2026-04-02T01:00:00Z", "reason": "Demo task replaced it"}}
path.write_text(update_spec_markdown(text, data), encoding="utf-8")
PY
  printf 'ok\n' > "$repo/drift.txt"
  write_review_pass "$repo" "review-drift" "drift.txt"
  printf 'drifted\n' > "$repo/drift.txt"

  echo "[6/6] report stays runnable and surfaces actionable triage"
  capture report_output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld report"
  assert_contains "$report_output" "Triage:" "report should include the triage heading"
  assert_contains "$report_output" "Stale drafts (>7d)" "report should include stale draft triage"
  assert_contains "$report_output" "stale-draft" "report should list stale drafts"
  assert_contains "$report_output" "Approved waiting to start" "report should include approved triage"
  assert_contains "$report_output" "waiting-spec" "report should list approved specs waiting to start"
  assert_contains "$report_output" "Stale active specs" "report should include stale active triage"
  assert_contains "$report_output" "stale-active" "report should list active done/open specs"
  assert_contains "$report_output" "Superseded specs" "report should include superseded triage"
  assert_contains "$report_output" "superseded-old -> demo-task" "report should list superseded specs"
  assert_contains "$report_output" "Active with no exec evidence" "report should include active/no-exec triage"
  assert_contains "$report_output" "no-exec" "report should list active specs without exec evidence"
  assert_contains "$report_output" "Review drift" "report should include review drift triage"
  assert_contains "$report_output" "review-drift" "report should list review drift specs"
  assert_contains "$report_output" "current workspace no longer matches the reviewed git state" "report should explain review drift"

  echo "PASS: lifecycle smoke"
}

main "$@"
