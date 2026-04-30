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
  repo="$(mktemp -d /tmp/scafld-prompt-precedence.XXXXXX)"
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

write_approved_spec() {
  local repo="$1"
  SCAFLD_SPEC_CREATED="2026-04-24T00:00:00Z" \
    write_markdown_spec "$repo/.scafld/specs/approved/agent-task.md" \
    "agent-task" "approved" "Prompt precedence smoke" \
    "agent.txt" "grep -q '^green$' agent.txt"
}

repo="$(new_repo)"
write_approved_spec "$repo"

echo "[1/2] workspace prompt overrides the managed reset copy"
cat > "$repo/.scafld/prompts/exec.md" <<'EOF'
# WORKSPACE EXEC MARKER

This prompt proves the workspace-owned template is active.
EOF

echo "[2/2] rendered handoff cites and contains the workspace prompt"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld handoff agent-task --phase phase1 --json"
assert_json "$output" "data['result']['template'] == '.scafld/prompts/exec.md'" "handoff should read the workspace prompt source"
assert_json "$output" "'WORKSPACE EXEC MARKER' in data['result']['content']" "rendered handoff should contain the workspace prompt marker"

echo "PASS: prompt precedence smoke"
