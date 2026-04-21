#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI="$REPO_ROOT/cli/scafld"
SYNC_VERSION="$REPO_ROOT/scripts/sync_version.py"
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

EXPECTED_VERSION="$(python3 "$CLI" --version | awk '{print $2}')"
DIST_DIR="$(mktemp -d /tmp/scafld-package-smoke.XXXXXX)"
TMP_DIRS+=("$DIST_DIR")

echo "[1/5] canonical version is in sync"
python3 "$SYNC_VERSION" --check >/dev/null || fail "version sync check failed"

echo "[2/5] build wheel"
python3 -m pip wheel "$REPO_ROOT" --no-deps -w "$DIST_DIR" >/dev/null || fail "wheel build failed"
WHEEL="$(find "$DIST_DIR" -maxdepth 1 -name "scafld-*.whl" | head -n 1)"
[ -n "$WHEEL" ] || fail "wheel not produced"

echo "[3/5] installed wheel exposes the expected version"
python3 -m venv "$DIST_DIR/venv"
# shellcheck disable=SC1091
source "$DIST_DIR/venv/bin/activate"
pip install --no-deps "$WHEEL" >/dev/null || fail "wheel install failed"
output="$(scafld --version)"
[[ "$output" == "scafld $EXPECTED_VERSION" ]] || fail "installed wheel reported '$output'"
python3 - <<PY || fail "installed wheel metadata is missing expected homepage"
from importlib import metadata

md = metadata.metadata("scafld")
assert md["Home-page"] == "https://0state.com/scafld", md["Home-page"]
assert md["Author"] == "0state", md["Author"]
project_urls = md.get_all("Project-URL") or []
assert "Documentation, https://0state.com/scafld/docs" in project_urls, project_urls
PY

echo "[4/5] installed wheel can init a workspace"
WS="$(mktemp -d /tmp/scafld-wheel-workspace.XXXXXX)"
TMP_DIRS+=("$WS")
(cd "$WS" && scafld init >/dev/null) || fail "installed wheel could not init"
python3 - <<PY || fail "installed wheel did not write managed runtime assets"
import json
from pathlib import Path

workspace = Path("$WS")
manifest = json.load(open(workspace / ".ai" / "scafld" / "manifest.json"))
assert manifest["scafld_version"] == "$EXPECTED_VERSION", manifest["scafld_version"]
assert (workspace / ".ai" / "scafld" / "prompts" / "harden.md").exists()
PY

echo "[5/5] npm pack dry-run exposes the expected tarball"
PACK_JSON="$(cd "$REPO_ROOT" && npm pack --json --dry-run --silent)"
PACK_JSON="$PACK_JSON" python3 - <<PY || fail "npm pack output did not match expected contents"
import json
import os

payload = json.loads(os.environ["PACK_JSON"])
pkg = payload[0]
assert pkg["name"] == "scafld", pkg["name"]
assert pkg["version"] == "$EXPECTED_VERSION", pkg["version"]
files = {entry["path"] for entry in pkg["files"]}
required = {
    "cli/scafld",
    ".ai/config.yaml",
    ".ai/prompts/harden.md",
    ".ai/schemas/spec.json",
    ".ai/specs/examples/add-error-codes.yaml",
    "scafld/_version.py",
    "AGENTS.md",
    "CLAUDE.md",
    "CONVENTIONS.md",
    "LICENSE",
    "README.md",
    "install.sh",
    "package.json",
}
missing = sorted(required - files)
assert not missing, missing
assert ".ai/scafld/manifest.json" not in files
PY

PACKAGE_JSON_PATH="$REPO_ROOT/package.json" EXPECTED_VERSION="$EXPECTED_VERSION" node - <<'JS' || fail "package.json metadata is missing expected homepage"
const pkg = require(process.env.PACKAGE_JSON_PATH);
if (pkg.homepage !== "https://0state.com/scafld") {
  throw new Error(`homepage=${pkg.homepage}`);
}
if (pkg.author !== "0state") {
  throw new Error(`author=${pkg.author}`);
}
if (pkg.version !== process.env.EXPECTED_VERSION) {
  throw new Error(`version=${pkg.version}`);
}
JS

echo "PASS: package smoke"
