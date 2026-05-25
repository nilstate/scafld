#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$root/dist"
max_bytes="${SCAFLD_MAX_RELEASE_BINARY_BYTES:-6291456}"

if [[ ! "$max_bytes" =~ ^[0-9]+$ ]]; then
  echo "SCAFLD_MAX_RELEASE_BINARY_BYTES must be an integer byte count" >&2
  exit 2
fi

count=0
total=0
failed=0
for file in "$dist"/scafld_*; do
  [[ -f "$file" ]] || continue
  name="${file##*/}"
  if [[ -n "$version" && "$name" != scafld_"$version"_* ]]; then
    continue
  fi
  size="$(wc -c < "$file" | tr -d ' ')"
  count=$((count + 1))
  total=$((total + size))
  printf 'release binary size: %s %s bytes (budget %s)\n' "$name" "$size" "$max_bytes"
  if (( size > max_bytes )); then
    failed=1
  fi
done

if (( count == 0 )); then
  echo "no release binaries found in $dist" >&2
  exit 1
fi

printf 'release binary size total: %s bytes across %s asset(s)\n' "$total" "$count"
if (( failed != 0 )); then
  echo "one or more release binaries exceeded SCAFLD_MAX_RELEASE_BINARY_BYTES=$max_bytes" >&2
  exit 1
fi
