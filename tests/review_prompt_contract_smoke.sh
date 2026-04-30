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
  repo="$(mktemp -d /tmp/scafld-review-contract.XXXXXX)"
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

write_active_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
  SCAFLD_SPEC_PHASE_STATUS="completed" \
  SCAFLD_SPEC_CRITERION_RESULT="pass" \
    write_markdown_spec "$repo/.scafld/specs/active/review-task.md" \
    "review-task" "in_progress" "Review prompt contract smoke" \
    "app.txt" "grep -q '^ok$' app.txt"
}

repo="$(new_repo)"
printf 'ok\n' > "$repo/app.txt"
write_active_spec "$repo"

echo "[1/2] source review prompt carries the stricter contract"
assert_contains_file "$REPO_ROOT/.scafld/prompts/review.md" 'Required finding format' "review prompt should teach the required finding format"
assert_contains_file "$REPO_ROOT/.scafld/prompts/review.md" 'No issues found — checked <specific files, callers, rules, or paths attacked>' "review prompt should teach the explicit no-issues format"
assert_contains_file "$REPO_ROOT/.scafld/prompts/review.md" 'untrusted data' "review prompt should treat spec content as untrusted data"

echo "[2/2] review scaffolding emits the required sections and format guidance"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld review review-task --json"
assert_json "$output" "'Regression Hunt' in data['result']['required_sections']" "review should require the regression hunt section"
assert_json "$output" "'Blocking' in data['result']['required_sections']" "review should require the blocking section"
assert_contains "$output" 'path/file.py:88' "review prompt should carry the strict finding example"
assert_contains "$output" '<specific files, callers, rules, or paths attacked>' "review prompt should carry the explicit no-issues guidance"

echo "PASS: review prompt contract smoke"
