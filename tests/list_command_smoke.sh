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
  repo="$(mktemp -d /tmp/scafld-list-command.XXXXXX)"
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

write_spec_fixture() {
  local repo="$1"
  local rel_path="$2"
  local task_id="$3"
  local status="$4"
  local title="$5"
  local phase_status="$6"

  mkdir -p "$(dirname "$repo/$rel_path")"
  SCAFLD_SPEC_CREATED="2026-04-25T00:00:00Z" \
  SCAFLD_SPEC_PHASE_STATUS="$phase_status" \
    write_markdown_spec "$repo/$rel_path" "$task_id" "$status" "$title" "fixture.txt" "true"
}

repo="$(new_repo)"

write_spec_fixture "$repo" ".scafld/specs/drafts/draft-task.md" "draft-task" "draft" "Draft task" "pending"
write_spec_fixture "$repo" ".scafld/specs/approved/approved-task.md" "approved-task" "approved" "Approved task" "pending"
write_spec_fixture "$repo" ".scafld/specs/active/active-task.md" "active-task" "in_progress" "Active task" "completed"
write_spec_fixture "$repo" ".scafld/specs/archive/2026-04/completed-task.md" "completed-task" "completed" "Completed task" "completed"
write_spec_fixture "$repo" ".scafld/specs/archive/2026-04/superseded-task.md" "superseded-task" "cancelled" "Superseded task" "completed"
PYTHONPATH="$REPO_ROOT" python3 - "$repo/.scafld/specs/archive/2026-04/superseded-task.md" <<'PY'
import sys
from pathlib import Path
from scafld.spec_markdown import parse_spec_markdown, update_spec_markdown
path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
data = parse_spec_markdown(text)
data["origin"] = {
    "origin": None,
    "supersession": {
        "superseded_by": "completed-task",
        "superseded_at": "2026-04-25T01:00:00Z",
        "reason": "Replaced by completed-task",
    },
    "sync": None,
}
path.write_text(update_spec_markdown(text, data), encoding="utf-8")
PY

echo "[1/3] unfiltered list shows lifecycle buckets without crashing"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list"; then
  fail "scafld list should succeed"
fi
assert_not_contains "$output" "Traceback" "list should not crash with a traceback"
assert_contains "$output" "drafts/" "list should show drafts bucket"
assert_contains "$output" "approved/" "list should show approved bucket"
assert_contains "$output" "active/" "list should show active bucket"
assert_contains "$output" "archive/2026-04/" "list should show archive bucket"
assert_contains "$output" "draft-task" "list should include draft task"
assert_contains "$output" "approved-task" "list should include approved task"
assert_contains "$output" "active-task" "list should include active task"
assert_contains "$output" "completed-task" "list should include archived task"
assert_contains "$output" "stale active" "list should mark active specs with all phases complete"
assert_contains "$output" "superseded by completed-task" "list should mark superseded specs"

echo "[2/3] filtered list narrows to the requested bucket"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list active"; then
  fail "scafld list active should succeed"
fi
assert_not_contains "$output" "Traceback" "filtered list should not crash with a traceback"
assert_contains "$output" "active/" "active filter should keep the active bucket"
assert_contains "$output" "active-task" "active filter should include the active task"
assert_not_contains "$output" "draft-task" "active filter should exclude draft task"
assert_not_contains "$output" "approved-task" "active filter should exclude approved task"
assert_not_contains "$output" "completed-task" "active filter should exclude archived task"

echo "[3/3] state filters expose stale active and superseded specs"
if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list stale-active"; then
  fail "scafld list stale-active should succeed"
fi
assert_contains "$output" "active-task" "stale-active filter should include active done/open specs"
assert_not_contains "$output" "draft-task" "stale-active filter should exclude drafts"

if ! capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld list superseded"; then
  fail "scafld list superseded should succeed"
fi
assert_contains "$output" "superseded-task" "superseded filter should include superseded specs"
assert_not_contains "$output" "active-task" "superseded filter should exclude stale-only specs"

echo "PASS: list command smoke"
