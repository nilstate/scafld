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
out_dir="${OUT_DIR:-$root/.stage/package-managers/rendered}"
checksums_file="${CHECKSUMS_FILE:-}"
tmp_dir=""

cleanup() {
  if [[ -n "$tmp_dir" ]]; then
    rm -rf "$tmp_dir"
  fi
}
trap cleanup EXIT

if [[ -z "$checksums_file" ]]; then
  if [[ -f "$root/dist/checksums.txt" ]] && grep -q "scafld_${version}_darwin_amd64" "$root/dist/checksums.txt"; then
    checksums_file="$root/dist/checksums.txt"
  else
    tmp_dir="$(mktemp -d)"
    checksums_file="$tmp_dir/checksums.txt"
    curl -fsSL \
      "https://github.com/nilstate/scafld/releases/download/v${version}/checksums.txt" \
      -o "$checksums_file"
  fi
fi

sha_for() {
  local file="$1"
  local sha
  sha="$(awk -v f="$file" '$2 == f {print $1}' "$checksums_file")"
  if [[ -z "$sha" ]]; then
    echo "missing checksum for $file in $checksums_file" >&2
    exit 1
  fi
  printf '%s' "$sha"
}

escape_replacement() {
  printf '%s' "$1" | sed -e 's/[\/&]/\\&/g'
}

render() {
  local template="$1"
  local target="$2"
  mkdir -p "$(dirname "$target")"
  sed \
    -e "s/{{VERSION}}/$(escape_replacement "$version")/g" \
    -e "s/{{SHA256_DARWIN_AMD64}}/$(escape_replacement "$sha_darwin_amd64")/g" \
    -e "s/{{SHA256_DARWIN_ARM64}}/$(escape_replacement "$sha_darwin_arm64")/g" \
    -e "s/{{SHA256_LINUX_AMD64}}/$(escape_replacement "$sha_linux_amd64")/g" \
    -e "s/{{SHA256_LINUX_ARM64}}/$(escape_replacement "$sha_linux_arm64")/g" \
    -e "s/{{SHA256_WINDOWS_AMD64}}/$(escape_replacement "$sha_windows_amd64")/g" \
    -e "s/{{SHA256_WINDOWS_ARM64}}/$(escape_replacement "$sha_windows_arm64")/g" \
    "$template" > "$target"
}

sha_darwin_amd64="$(sha_for "scafld_${version}_darwin_amd64")"
sha_darwin_arm64="$(sha_for "scafld_${version}_darwin_arm64")"
sha_linux_amd64="$(sha_for "scafld_${version}_linux_amd64")"
sha_linux_arm64="$(sha_for "scafld_${version}_linux_arm64")"
sha_windows_amd64="$(sha_for "scafld_${version}_windows_amd64.exe")"
sha_windows_arm64="$(sha_for "scafld_${version}_windows_arm64.exe")"

rm -rf "$out_dir"
mkdir -p "$out_dir"

render "$root/package/homebrew/scafld.rb.tmpl" "$out_dir/homebrew/scafld.rb"
render "$root/package/scoop/scafld.json.tmpl" "$out_dir/scoop/scafld.json"
render "$root/package/winget/scafld.yaml.tmpl" "$out_dir/winget/0state.scafld.locale.en-US.yaml"
render "$root/package/winget/scafld.installer.yaml.tmpl" "$out_dir/winget/0state.scafld.installer.yaml"
render "$root/package/winget/scafld.version.yaml.tmpl" "$out_dir/winget/0state.scafld.yaml"

ruby -c "$out_dir/homebrew/scafld.rb" >/dev/null
jq empty "$out_dir/scoop/scafld.json"

if grep -R -n -E '\{\{[A-Z0-9_]+\}\}' "$out_dir" >/dev/null; then
  echo "rendered package-manager manifests still contain template markers" >&2
  exit 1
fi

echo "Rendered package-manager manifests: $out_dir"
