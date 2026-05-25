#!/usr/bin/env bash
set -euo pipefail

version="${1:-0.0.0-snapshot}"
version="${version#v}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$root/dist"

case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
  darwin) goos="darwin" ;;
  linux) goos="linux" ;;
  *) echo "unsupported installer smoke OS: $(uname -s)" >&2; exit 2 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported installer smoke architecture: $(uname -m)" >&2; exit 2 ;;
esac

asset="scafld_${version}_${goos}_${goarch}"
if [[ ! -x "$dist/$asset" ]] || ! grep -q " $asset\$" "$dist/checksums.txt" 2>/dev/null; then
  "$root/scripts/build-release-artifacts.sh" "$version"
fi

base_url="$(
  DIST="$dist" python3 - <<'PY'
import os
from pathlib import Path
print(Path(os.environ["DIST"]).resolve().as_uri())
PY
)"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

cp -R "$root/package/npm" "$tmp/npm"
env \
  SCAFLD_INSTALL_BASE_URL="$base_url" \
  SCAFLD_INSTALL_VERSION="$version" \
  node "$tmp/npm/lib/install.js"
npm_version="$(
  env \
    SCAFLD_INSTALL_BASE_URL="$base_url" \
    SCAFLD_INSTALL_VERSION="$version" \
    node "$tmp/npm/bin/scafld.js" --version
)"
if [[ "$npm_version" != "$version" ]]; then
  echo "npm launcher returned $npm_version, want $version" >&2
  exit 1
fi

py_version="$(
  env \
    PYTHONPATH="$root/package/pypi/src" \
    SCAFLD_INSTALL_BASE_URL="$base_url" \
    SCAFLD_INSTALL_VERSION="$version" \
    SCAFLD_INSTALL_DIR="$tmp/pypi-bin" \
    python3 -c 'from scafld_launcher.cli import main; raise SystemExit(main())' --version
)"
if [[ "$py_version" != "$version" ]]; then
  echo "PyPI launcher returned $py_version, want $version" >&2
  exit 1
fi

echo "release installer smoke passed for $asset"
