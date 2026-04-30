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
  repo="$(mktemp -d /tmp/scafld-mixed-detection.XXXXXX)"
  TMP_DIRS+=("$repo")
  (
    cd "$repo"
    git init -b main >/dev/null 2>&1
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    mkdir -p tests
    cat > package.json <<'EOF'
{
  "name": "mixed-fixture",
  "packageManager": "npm@11.0.0",
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "test": "vitest run",
    "lint": "eslint .",
    "typecheck": "tsc --noEmit"
  },
  "dependencies": {
    "react": "^19.0.0"
  }
}
EOF
    : > package-lock.json
    cat > tsconfig.json <<'EOF'
{
  "compilerOptions": {
    "strict": true
  }
}
EOF
    cat > pyproject.toml <<'EOF'
[project]
name = "mixed-fixture"
version = "0.1.0"

[tool.pytest.ini_options]
addopts = "-q"
EOF
    : > uv.lock
    scafld_cmd init >/dev/null
  )
  printf '%s\n' "$repo"
}

repo="$(new_repo)"

echo "[1/3] init merges mixed repo signals into one concrete config overlay"
assert_contains_file "$repo/.scafld/config.local.yaml" 'Detection: Mixed repo detected: Node (npm), React, TypeScript + Python (uv)' "mixed fixture should record mixed detection"
assert_contains_file "$repo/.scafld/config.local.yaml" 'compile_check: "npm run build && uv run python -m compileall ."' "mixed fixture should combine compile commands"
assert_contains_file "$repo/.scafld/config.local.yaml" 'targeted_tests: "npm test && uv run pytest"' "mixed fixture should combine targeted tests"
assert_contains_file "$repo/.scafld/config.local.yaml" 'full_test_suite: "npm test && uv run pytest"' "mixed fixture should combine full tests"
assert_contains_file "$repo/.scafld/config.local.yaml" 'linter_suite: "npm run lint"' "mixed fixture should keep the node linter"
assert_contains_file "$repo/.scafld/config.local.yaml" 'typecheck: "npm run typecheck"' "mixed fixture should keep the node typecheck"

echo "[2/3] plan surfaces the same mixed context through the taught agent workflow"
capture output bash -lc "cd '$repo' && PATH='$CLI_ROOT':\"\$PATH\" scafld plan mixed-task -t 'Mixed task' -s small -r low --json"
assert_json "$output" "data['state']['status'] == 'draft'" "plan should create a draft"
assert_json "$output" "data['state']['harden_status'] == 'in_progress'" "plan should open harden"
assert_json "$output" "data['result']['repo_context']['summary'] == 'Mixed repo detected: Node (npm), React, TypeScript + Python (uv)'" "plan should return the mixed repo summary"

echo "[3/3] draft scaffolding carries the mixed summary and concrete commands"
assert_contains_file "$repo/.scafld/specs/drafts/mixed-task.md" 'repo detected: Node (npm), React, TypeScript + Python (uv).' "mixed draft should record the mixed summary"
assert_contains_file "$repo/.scafld/specs/drafts/mixed-task.md" 'npm run build && uv run python -m compileall .' "mixed draft should inherit the combined compile command"
assert_contains_file "$repo/.scafld/specs/drafts/mixed-task.md" 'npm test && uv run pytest' "mixed draft should inherit the combined test command"

echo "PASS: mixed repo detection smoke"
