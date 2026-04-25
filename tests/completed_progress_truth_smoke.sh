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
  repo="$(mktemp -d /tmp/scafld-completed-progress.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    scafld_cmd init >/dev/null
    git add .
    git commit -m "init" >/dev/null 2>&1
  )
  printf '%s\n' "$repo"
}

write_legacy_archive_spec() {
  local repo="$1"
  mkdir -p "$repo/.ai/specs/archive/2026-04"
  cat > "$repo/.ai/specs/archive/2026-04/legacy-completed.yaml" <<'EOF'
spec_version: "1.1"
task_id: "legacy-completed"
created: "2026-04-01T00:00:00Z"
updated: "2026-04-01T00:00:00Z"
status: "completed"
task:
  title: "Legacy completed"
  summary: "Legacy archive with stale pending phase status"
  size: "small"
  risk_level: "low"
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Legacy done item"
        status: "pending"
planning_log:
  - timestamp: "2026-04-01T00:00:00Z"
    actor: "user"
    summary: "Legacy fixture"
phases:
  - id: "phase1"
    name: "Legacy"
    objective: "Legacy fixture"
    changes:
      - file: "legacy.txt"
        action: "update"
        content_spec: "Legacy"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "Legacy pass"
        command: "true"
        expected: "exit code 0"
        result: "pass"
    status: "pending"
EOF
}

write_active_spec() {
  local repo="$1"
  cat > "$repo/.ai/specs/active/fresh-completed.yaml" <<'EOF'
spec_version: "1.1"
task_id: "fresh-completed"
created: "2026-04-25T00:00:00Z"
updated: "2026-04-25T00:00:00Z"
status: "in_progress"
task:
  title: "Fresh completed"
  summary: "Exercise completion truth stamping"
  size: "small"
  risk_level: "low"
  acceptance:
    definition_of_done:
      - id: "dod1"
        description: "Fresh done item"
        status: "pending"
planning_log:
  - timestamp: "2026-04-25T00:00:00Z"
    actor: "user"
    summary: "Fresh fixture"
phases:
  - id: "phase1"
    name: "Fresh"
    objective: "Fresh fixture"
    changes:
      - file: "fresh.txt"
        action: "update"
        content_spec: "Fresh"
    acceptance_criteria:
      - id: "ac1_1"
        type: "custom"
        description: "Fresh pass"
        command: "true"
        expected: "exit code 0"
        result: "pass"
    status: "pending"
EOF
}

complete_scaffolded_review_round() {
  local repo="$1"
  local task_id="$2"
  REVIEW_REPO="$repo" REVIEW_TASK_ID="$task_id" python3 - <<'PY'
import json
import os
import pathlib
import re

repo = pathlib.Path(os.environ["REVIEW_REPO"])
task_id = os.environ["REVIEW_TASK_ID"]
review_path = repo / ".ai" / "reviews" / f"{task_id}.md"
text = review_path.read_text()

json_blocks = list(re.finditer(r"```json\s*\n(.*?)\n```", text, re.DOTALL))
if not json_blocks:
    raise SystemExit("review metadata JSON block not found")

metadata_match = json_blocks[-1]
metadata = json.loads(metadata_match.group(1))
metadata["round_status"] = "completed"
metadata["reviewer_mode"] = "fresh_agent"
metadata["reviewer_session"] = "smoke-review"
for pass_id in ("spec_compliance", "scope_drift", "regression_hunt", "convention_check", "dark_patterns"):
    metadata["pass_results"][pass_id] = "pass"
text = text[:metadata_match.start(1)] + json.dumps(metadata, indent=2) + text[metadata_match.end(1):]

pass_lines = "\n".join([
    "- spec_compliance: PASS",
    "- scope_drift: PASS",
    "- regression_hunt: PASS",
    "- convention_check: PASS",
    "- dark_patterns: PASS",
]) + "\n"

section_updates = {
    "Pass Results": pass_lines,
    "Regression Hunt": "No issues found — checked the legacy archive projection and fresh completion path.\n",
    "Convention Check": "No issues found — checked the lifecycle and review surfaces only.\n",
    "Dark Patterns": "No issues found — checked for stale archive truth and silent progress drift.\n",
    "Blocking": "No issues found — checked the archive truth surface.\n",
    "Non-blocking": "No issues found — checked for residual progress mismatches within the bounded task.\n",
    "Verdict": "pass\n",
}

for heading, body in section_updates.items():
    pattern = rf"(^### {re.escape(heading)}\s*\n?)(.*?)(?=^### |\Z)"
    text, count = re.subn(
        pattern,
        lambda match, body=body: (match.group(1) if match.group(1).endswith("\n") else match.group(1) + "\n") + body,
        text,
        count=1,
        flags=re.MULTILINE | re.DOTALL,
    )
    if count != 1:
        raise SystemExit(f"could not update section {heading}")

review_path.write_text(text)
PY
}

commit_fixtures() {
  local repo="$1"
  (
    cd "$repo"
    git add .
    git commit -m "fixture state" >/dev/null 2>&1
  )
}

archive_spec_path() {
  local repo="$1"
  local task_id="$2"
  find "$repo/.ai/specs/archive" -name "$task_id.yaml" -print | head -n 1
}

repo="$(new_repo)"
write_legacy_archive_spec "$repo"
write_active_spec "$repo"
commit_fixtures "$repo"
printf 'fresh\n' > "$repo/fresh.txt"

echo "[1/5] legacy completed archive lists as fully done"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list"
assert_contains "$output" "legacy-completed" "legacy completed fixture should appear in list output"
assert_contains "$output" "[1/1]" "legacy completed fixture should render truthful completed progress"

echo "[2/5] review opens for the fresh active spec"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review fresh-completed --json"
assert_json "$output" "data['ok'] is True and data['state']['review_action'] == 'opened'" "review should open for the fresh active spec"
complete_scaffolded_review_round "$repo" "fresh-completed"

echo "[3/5] complete archives the fresh spec through the raw CLI"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld complete fresh-completed --json"
assert_json "$output" "data['ok'] is True and data['state']['status'] == 'completed'" "complete should archive the fresh spec"

echo "[4/5] archived fresh spec stores terminal phase and DoD truth"
archive_path="$(archive_spec_path "$repo" "fresh-completed")"
[ -n "$archive_path" ] || fail "fresh-completed archive path should exist"
capture output bash -lc "python3 -c \"import yaml; from pathlib import Path; data = yaml.safe_load(Path('$archive_path').read_text()); assert data['phases'][0]['status'] == 'completed'; assert data['task']['acceptance']['definition_of_done'][0]['status'] == 'done'; print('ok')\""
assert_contains "$output" "ok" "archived fresh spec should store terminal completion truth"

echo "[5/5] list shows the newly archived spec as fully done too"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list"
assert_contains "$output" "fresh-completed" "fresh completed archive should appear in list output"
assert_contains "$output" "[1/1]" "fresh completed archive should render truthful completed progress"

echo "PASS: completed progress truth smoke"
