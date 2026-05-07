#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rendered="${RENDERED_DIR:-$root/.stage/package-managers/rendered}"
bucket="${SCOOP_BUCKET_DIR:?Set SCOOP_BUCKET_DIR to a checked-out nilstate/scoop-bucket repository}"
src="$rendered/scoop/scafld.json"
dst="$bucket/bucket/scafld.json"

if [[ ! -f "$src" ]]; then
  echo "rendered Scoop manifest missing: $src" >&2
  exit 1
fi
if [[ ! -d "$bucket/.git" ]]; then
  echo "Scoop bucket is not a git checkout: $bucket" >&2
  exit 1
fi

mkdir -p "$(dirname "$dst")"
cp "$src" "$dst"
echo "Published Scoop manifest to $dst"
