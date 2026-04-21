#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI="$REPO_ROOT/cli/scafld"
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

assert_contains_file() {
  local file="$1"
  local needle="$2"
  local message="$3"
  grep -Fq -- "$needle" "$file" || fail "$message"
}

run_scafld() {
  local repo="$1"
  shift
  (cd "$repo" && python3 "$CLI" "$@")
}

new_workspace() {
  local repo="$1"
  mkdir -p "$repo"
  (
    cd "$repo"
    git init >/dev/null 2>&1
    git branch -M main >/dev/null 2>&1 || true
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    python3 "$CLI" init >/dev/null
    python3 "$CLI" new t1 -t "Update smoke" -s small -r low >/dev/null
  )
}

new_node_fixture() {
  local repo="$1"
  mkdir -p "$repo"
  (
    cd "$repo"
    git init >/dev/null 2>&1
    git branch -M main >/dev/null 2>&1 || true
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    cat > package.json <<'EOF'
{
  "name": "node-fixture",
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
    python3 "$CLI" init >/dev/null
  )
}

new_python_fixture() {
  local repo="$1"
  mkdir -p "$repo"
  (
    cd "$repo"
    git init >/dev/null 2>&1
    git branch -M main >/dev/null 2>&1 || true
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    mkdir -p tests
    cat > pyproject.toml <<'EOF'
[project]
name = "python-fixture"
version = "0.1.0"
dependencies = ["fastapi"]

[tool.pytest.ini_options]
addopts = "-q"

[tool.ruff]
line-length = 100

[tool.mypy]
python_version = "3.12"
EOF
    : > uv.lock
    python3 "$CLI" init >/dev/null
  )
}

new_fallback_fixture() {
  local repo="$1"
  mkdir -p "$repo"
  (
    cd "$repo"
    git init >/dev/null 2>&1
    git branch -M main >/dev/null 2>&1 || true
    git config user.email smoke@example.com
    git config user.name "Smoke Test"
    python3 "$CLI" init >/dev/null
  )
}

ROOT="$(mktemp -d /tmp/scafld-update-smoke.XXXXXX)"
TMP_DIRS+=("$ROOT")
WS1="$ROOT/ws1"
WS2="$ROOT/ws2"
NODE_REPO="$ROOT/node-fixture"
PYTHON_REPO="$ROOT/python-fixture"
FALLBACK_REPO="$ROOT/fallback-fixture"
EXPECTED_VERSION="$(python3 "$CLI" --version | awk '{print $2}')"

echo "[1/8] create workspaces and init-detection fixtures"
new_workspace "$WS1"
new_workspace "$WS2"
new_node_fixture "$NODE_REPO"
new_python_fixture "$PYTHON_REPO"
new_fallback_fixture "$FALLBACK_REPO"
rm -rf "$WS1/.ai/scafld" "$WS2/.ai/scafld"
rm -f "$WS1/.ai/prompts/harden.md"
rm -f "$WS1/.ai/schemas/spec.json"
printf '\ncustom_marker: "keep-me"\n' >> "$WS1/.ai/config.yaml"

echo "[2/8] init detects a Node toolchain and writes concrete commands"
assert_contains_file "$NODE_REPO/.ai/config.local.yaml" 'Detection: Node repo detected (npm), React, TypeScript' "node fixture should record the detected stack"
assert_contains_file "$NODE_REPO/.ai/config.local.yaml" 'compile_check: "npm run build"' "node fixture should suggest the build command"
assert_contains_file "$NODE_REPO/.ai/config.local.yaml" 'targeted_tests: "npm test"' "node fixture should suggest the test command"
assert_contains_file "$NODE_REPO/.ai/config.local.yaml" 'linter_suite: "npm run lint"' "node fixture should suggest the lint command"
assert_contains_file "$NODE_REPO/.ai/config.local.yaml" 'typecheck: "npm run typecheck"' "node fixture should suggest the typecheck command"

echo "[3/8] init detects a Python toolchain and writes concrete commands"
assert_contains_file "$PYTHON_REPO/.ai/config.local.yaml" 'Detection: Python repo detected (uv), FastAPI' "python fixture should record the detected stack"
assert_contains_file "$PYTHON_REPO/.ai/config.local.yaml" 'compile_check: "uv run python -m compileall ."' "python fixture should suggest a compile check"
assert_contains_file "$PYTHON_REPO/.ai/config.local.yaml" 'targeted_tests: "uv run pytest"' "python fixture should suggest pytest"
assert_contains_file "$PYTHON_REPO/.ai/config.local.yaml" 'linter_suite: "uv run ruff check ."' "python fixture should suggest ruff"
assert_contains_file "$PYTHON_REPO/.ai/config.local.yaml" 'typecheck: "uv run mypy ."' "python fixture should suggest mypy"

echo "[4/8] unknown repos keep the safe placeholder fallback"
assert_contains_file "$FALLBACK_REPO/.ai/config.local.yaml" 'Detection: no known Node or Python repo markers found' "fallback fixture should say autodetection fell back"
assert_contains_file "$FALLBACK_REPO/.ai/config.local.yaml" "compile_check: \"echo 'Replace: your build command'\"" "fallback fixture should keep placeholder commands"

echo "[5/8] scafld update --scan-root recreates managed bundles"
python3 "$CLI" update --scan-root "$ROOT" >/dev/null || fail "scan-root update failed"
[ -f "$WS1/.ai/scafld/manifest.json" ] || fail "ws1 missing managed manifest"
[ -f "$WS2/.ai/scafld/manifest.json" ] || fail "ws2 missing managed manifest"

echo "[6/8] manifest records the current scafld version"
python3 - <<PY || fail "manifest missing expected version or managed assets"
import json

for path in ("$WS1/.ai/scafld/manifest.json", "$WS2/.ai/scafld/manifest.json"):
    data = json.load(open(path))
    assert data["scafld_version"] == "$EXPECTED_VERSION", data["scafld_version"]
    assert ".ai/scafld/prompts/harden.md" in data["managed_assets"], data["managed_assets"].keys()
    assert ".ai/scafld/schemas/spec.json" in data["managed_assets"], data["managed_assets"].keys()
PY

echo "[7/8] repo-owned config stays intact"
grep -q 'custom_marker: "keep-me"' "$WS1/.ai/config.yaml" \
  || fail "update overwrote repo-specific config"

echo "[8/8] managed prompt and schema power legacy workspaces"
validate_output="$(run_scafld "$WS1" validate t1 2>&1 || true)"
[[ "$validate_output" != *"schema not found"* ]] || fail "validate did not use managed schema"
[[ "$validate_output" == *"TODO placeholder"* ]] || fail "validate did not reach schema-backed validation"
output="$(run_scafld "$WS1" harden t1)"
[[ "$output" == *"HARDEN MODE"* ]] || fail "harden did not use managed prompt"

echo "PASS: update smoke"
