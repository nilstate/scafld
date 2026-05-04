#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
if [[ -z "$version" ]]; then
  version="$(git describe --tags --abbrev=0 2>/dev/null || true)"
fi
version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: $0 <semver>" >&2
  exit 2
fi

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
dist="$root/dist"
rm -rf "$dist"
mkdir -p "$dist"

targets=(
  "darwin amd64"
  "darwin arm64"
  "linux amd64"
  "linux arm64"
  "windows amd64"
  "windows arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  ext=""
  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
  fi
  asset="scafld_${version}_${goos}_${goarch}${ext}"
  echo "building $asset"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w -X github.com/nilstate/scafld/v2/internal/adapters/cli.version=${version}" \
    -o "$dist/$asset" \
    "$root/cmd/scafld"
done

(
  cd "$dist"
  shasum -a 256 scafld_* > checksums.txt
)

{
  printf '{\n'
  printf '  "version": "%s",\n' "$version"
  printf '  "repository": "github.com/nilstate/scafld",\n'
  printf '  "assets": [\n'
  first=1
  while read -r sum file; do
    if [[ "$file" == "checksums.txt" ]]; then
      continue
    fi
    if [[ $first -eq 0 ]]; then
      printf ',\n'
    fi
    first=0
    IFS='_' read -r _ asset_version goos arch_ext <<<"$file"
    goarch="${arch_ext%.exe}"
    printf '    {"name":"%s","goos":"%s","goarch":"%s","sha256":"%s"}' "$file" "$goos" "$goarch" "$sum"
  done < "$dist/checksums.txt"
  printf '\n  ]\n'
  printf '}\n'
} > "$dist/manifest.json"
