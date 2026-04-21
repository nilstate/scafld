#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CLI="$REPO_ROOT/cli/scafld"
SYNC_VERSION="$REPO_ROOT/scripts/sync_version.py"
TMP_DIRS=()
source "$SCRIPT_DIR/smoke_lib.sh"

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
requires = md.get_all("Requires-Dist") or []
assert any(req.startswith("PyYAML") for req in requires), requires
PY

echo "[4/6] installed wheel can init a workspace"
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

echo "[5/6] installed wheel init is idempotent for the managed manifest"
before_manifest="$(cat "$WS/.ai/scafld/manifest.json")"
(cd "$WS" && scafld init >/dev/null) || fail "installed wheel could not re-init an existing workspace"
after_manifest="$(cat "$WS/.ai/scafld/manifest.json")"
[[ "$before_manifest" == "$after_manifest" ]] || fail "re-running init rewrote the managed manifest"

echo "[6/6] npm pack dry-run exposes the expected tarball"
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
    "scafld/__main__.py",
    "scafld/command_runtime.py",
    "scafld/config.py",
    "scafld/error_codes.py",
    "scafld/errors.py",
    "scafld/output.py",
    "scafld/projections.py",
    "scafld/reviewing.py",
    "scafld/spec_store.py",
    "scafld/spec_templates.py",
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
