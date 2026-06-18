#!/usr/bin/env sh
set -eu

receipt="${SCAFLD_RECEIPT_PATH:-${1:-}}"
target="${SCAFLD_VERIFY_TARGET:-${2:-}}"
trusted_keys="${SCAFLD_TRUSTED_KEYS:-}"
version="${SCAFLD_VERSION:-v2.4.8}"

if [ -z "$target" ] || [ "$target" = "0000000000000000000000000000000000000000" ]; then
  echo "error: SCAFLD_VERIFY_TARGET must be a base commit sha or ref" >&2
  exit 2
fi

if [ -z "$trusted_keys" ]; then
  tmp_dir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}"
  mkdir -p "$tmp_dir"
  trusted_keys="$tmp_dir/scafld-trusted-keys.json"
  if ! git show "$target:.scafld/trusted-keys.json" > "$trusted_keys"; then
    echo "error: could not load .scafld/trusted-keys.json from verify target $target" >&2
    echo "       bootstrap the trusted key on the protected branch before enabling the merge gate" >&2
    exit 2
  fi
fi

if [ -z "$receipt" ]; then
  receipts="$(git diff --name-only "$target"...HEAD -- '.scafld/receipts/*.json' 2>/dev/null | grep -v '^.scafld/receipts/latest\.json$' || true)"
  count="$(printf '%s\n' "$receipts" | sed '/^$/d' | wc -l | tr -d ' ')"
  if [ "$count" = "1" ]; then
    receipt="$receipts"
  elif [ "$count" = "0" ] && [ -z "${CI:-}" ] && [ -f ".scafld/receipts/latest.json" ]; then
    receipt=".scafld/receipts/latest.json"
  else
    echo "error: expected exactly one changed task receipt, found $count" >&2
    printf '%s\n' "$receipts" >&2
    exit 2
  fi
fi

if ! command -v scafld >/dev/null 2>&1; then
  if ! command -v go >/dev/null 2>&1; then
    echo "error: scafld is not on PATH and go is unavailable to install scafld@$version" >&2
    exit 2
  fi
  gobin="$(go env GOBIN)"
  if [ -z "$gobin" ]; then
    gobin="$(go env GOPATH)/bin"
  fi
  export PATH="$gobin:$PATH"
  go install "github.com/nilstate/scafld/v2/cmd/scafld@$version"
fi

scafld verify "$receipt" --target "$target" --trusted-keys "$trusted_keys"
