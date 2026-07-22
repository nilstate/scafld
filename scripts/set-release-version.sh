#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: $0 <semver>" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

node -e '
const fs = require("node:fs");
const version = process.argv[1];
const path = "package/npm/package.json";
const pkg = JSON.parse(fs.readFileSync(path, "utf8"));
pkg.version = version;
fs.writeFileSync(path, `${JSON.stringify(pkg, null, 2)}\n`);
' "$version"

perl -0pi -e "s/^version = \"[^\"]+\"/version = \"$version\"/m" "$root/package/pypi/pyproject.toml"
perl -0pi -e "s/__version__ = \"[^\"]+\"/__version__ = \"$version\"/m" "$root/package/pypi/src/scafld_launcher/__init__.py"
perl -0pi -e "s/SCAFLD_VERSION:-v[0-9][0-9A-Za-z.-]*/SCAFLD_VERSION:-v$version/g" \
  "$root/scripts/scafld-verify.sh" \
  "$root/internal/adapters/corebundle/assets/initwire/scripts/scafld-verify.sh"
perl -0pi -e "s/default: v[0-9][0-9A-Za-z.-]*/default: v$version/g" \
  "$root/.github/actions/scafld-verify/action.yml"
perl -0pi -e "s/SCAFLD_VERSION: v[0-9][0-9A-Za-z.-]*/SCAFLD_VERSION: v$version/g" \
  "$root/.github/workflows/scafld-verify.yml" \
  "$root/internal/adapters/corebundle/assets/initwire/ci/scafld-verify.yml"
