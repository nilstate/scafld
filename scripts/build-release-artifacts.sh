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

cleanup_windows_resources() {
  rm -f "$root/cmd/scafld/rsrc_windows_amd64.syso" "$root/cmd/scafld/rsrc_windows_arm64.syso"
}

windows_file_version() {
  local base="$1"
  base="${base%%[-+]*}"
  IFS='.' read -r major minor patch extra <<<"$base"
  printf '%s.%s.%s.%s' "${major:-0}" "${minor:-0}" "${patch:-0}" "${extra:-0}"
}

generate_windows_resources() {
  local resource_version
  resource_version="$(windows_file_version "$version")"
  local winres_json
  winres_json="$(mktemp)"
  cat > "$winres_json" <<JSON
{
  "RT_MANIFEST": {
    "#1": {
      "0409": {
        "identity": {
          "name": "0state.scafld",
          "version": "$resource_version"
        },
        "description": "scafld CLI",
        "minimum-os": "win7",
        "execution-level": "as invoker",
        "ui-access": false,
        "auto-elevate": false,
        "dpi-awareness": "per monitor v2",
        "long-path-aware": true
      }
    }
  },
  "RT_VERSION": {
    "#1": {
      "0000": {
        "fixed": {
          "file_version": "$resource_version",
          "product_version": "$resource_version"
        },
        "info": {
          "0409": {
            "CompanyName": "0state",
            "FileDescription": "scafld CLI",
            "FileVersion": "$version",
            "InternalName": "scafld",
            "LegalCopyright": "Copyright (c) 0state",
            "OriginalFilename": "scafld.exe",
            "ProductName": "scafld",
            "ProductVersion": "$version"
          }
        }
      }
    }
  }
}
JSON
  go run github.com/tc-hib/go-winres@v0.3.3 make \
    --in "$winres_json" \
    --arch amd64,arm64 \
    --out "$root/cmd/scafld/rsrc"
  rm -f "$winres_json"
}

cleanup_windows_resources
trap cleanup_windows_resources EXIT
generate_windows_resources

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
