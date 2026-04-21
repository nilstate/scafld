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

ROOT="$(mktemp -d /tmp/scafld-update-smoke.XXXXXX)"
TMP_DIRS+=("$ROOT")
WS1="$ROOT/ws1"
WS2="$ROOT/ws2"
EXPECTED_VERSION="$(python3 "$CLI" --version | awk '{print $2}')"

echo "[1/5] create workspaces and simulate legacy state"
new_workspace "$WS1"
new_workspace "$WS2"
rm -rf "$WS1/.ai/scafld" "$WS2/.ai/scafld"
rm -f "$WS1/.ai/prompts/harden.md"
rm -f "$WS1/.ai/schemas/spec.json"
printf '\ncustom_marker: "keep-me"\n' >> "$WS1/.ai/config.yaml"

echo "[2/5] scafld update --scan-root recreates managed bundles"
python3 "$CLI" update --scan-root "$ROOT" >/dev/null || fail "scan-root update failed"
[ -f "$WS1/.ai/scafld/manifest.json" ] || fail "ws1 missing managed manifest"
[ -f "$WS2/.ai/scafld/manifest.json" ] || fail "ws2 missing managed manifest"

echo "[3/5] manifest records the current scafld version"
python3 - <<PY || fail "manifest missing expected version or managed assets"
import json

for path in ("$WS1/.ai/scafld/manifest.json", "$WS2/.ai/scafld/manifest.json"):
    data = json.load(open(path))
    assert data["scafld_version"] == "$EXPECTED_VERSION", data["scafld_version"]
    assert ".ai/scafld/prompts/harden.md" in data["managed_assets"], data["managed_assets"].keys()
    assert ".ai/scafld/schemas/spec.json" in data["managed_assets"], data["managed_assets"].keys()
PY

echo "[4/5] repo-owned config stays intact"
grep -q 'custom_marker: "keep-me"' "$WS1/.ai/config.yaml" \
  || fail "update overwrote repo-specific config"

echo "[5/5] managed prompt and schema power legacy workspaces"
validate_output="$(run_scafld "$WS1" validate t1 2>&1 || true)"
[[ "$validate_output" != *"schema not found"* ]] || fail "validate did not use managed schema"
[[ "$validate_output" == *"TODO placeholder"* ]] || fail "validate did not reach schema-backed validation"
output="$(run_scafld "$WS1" harden t1)"
[[ "$output" == *"HARDEN MODE"* ]] || fail "harden did not use managed prompt"

echo "PASS: update smoke"
