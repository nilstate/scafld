#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rendered="${RENDERED_DIR:-$root/.stage/package-managers/rendered}"
tap="${TAP_REPO_DIR:?Set TAP_REPO_DIR to a checked-out nilstate/homebrew-tap repository}"
src="$rendered/homebrew/scafld.rb"
dst="$tap/Formula/scafld.rb"

if [[ ! -f "$src" ]]; then
  echo "rendered Homebrew formula missing: $src" >&2
  exit 1
fi
if [[ ! -d "$tap/.git" ]]; then
  echo "Homebrew tap is not a git checkout: $tap" >&2
  exit 1
fi

mkdir -p "$(dirname "$dst")"
cp "$src" "$dst"
echo "Published Homebrew formula to $dst"
