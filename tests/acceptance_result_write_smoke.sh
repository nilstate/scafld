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
  repo="$(mktemp -d /tmp/scafld-acceptance-write.XXXXXX)"
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

write_active_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-25T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/active/rewrite-task.md" \
    "rewrite-task" "in_progress" "Acceptance writer smoke" \
    "fixture.txt" "python3 -c \"print('alpha ' * 40)\""
  python3 - "$repo/.scafld/specs/active/rewrite-task.md" <<'PY'
import sys
from pathlib import Path
path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8")
path.write_text(text.replace("Fixture summary.", "Fixture summary.\n\nkeep-me", 1), encoding="utf-8")
PY
}

repo="$(new_repo)"
mkdir -p "$repo/.scafld/specs/active"
write_active_spec "$repo"

echo "[1/4] first explicit exec succeeds"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec rewrite-task --phase phase1 --json"
assert_json "$output" "data['ok'] is True and data['result']['criteria'][0]['status'] == 'pass'" "first explicit exec should pass"

echo "[2/4] second explicit exec also succeeds"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld exec rewrite-task --phase phase1 --json"
assert_json "$output" "data['ok'] is True and data['result']['criteria'][0]['status'] == 'pass'" "second explicit exec should still pass"

echo "[3/4] resulting spec keeps author prose and runner result state"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" PYTHONPATH='$REPO_ROOT' python3 -c \"from pathlib import Path; from scafld.spec_markdown import parse_spec_markdown; path = Path('.scafld/specs/active/rewrite-task.md'); text = path.read_text(); assert 'keep-me' in text; data = parse_spec_markdown(text); criterion = data['phases'][0]['acceptance_criteria'][0]; assert criterion['result'] == 'pass'; assert criterion['status'] == 'pass'; assert 'alpha' in criterion['evidence']; print('ok')\""
assert_contains "$output" "ok" "active spec should remain valid Markdown after repeated exec"

echo "[4/4] packaged validate still passes after repeated exec"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld validate rewrite-task --json"
assert_json "$output" "data['ok'] is True and data['result']['valid'] is True" "validate should still succeed after repeated exec"

echo "PASS: acceptance result write smoke"
