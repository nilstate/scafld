#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
winget_repo="${2:-}"

if [[ -z "$version" || -z "$winget_repo" ]]; then
  echo "usage: $0 <semver> <path-to-winget-pkgs-checkout>" >&2
  exit 2
fi

version="${version#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid semver: $version" >&2
  exit 2
fi

if [[ ! -d "$winget_repo/.git" ]]; then
  echo "not a git checkout: $winget_repo" >&2
  exit 2
fi

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

base_url="https://github.com/nilstate/scafld/releases/download/v${version}"
rendered_archive="$tmp_dir/package-managers-rendered.tar.gz"
checksums_file="$tmp_dir/checksums.txt"

curl -fsSL "$base_url/package-managers-rendered.tar.gz" -o "$rendered_archive"
curl -fsSL "$base_url/checksums.txt" -o "$checksums_file"
tar -C "$tmp_dir" -xzf "$rendered_archive"

manifest_dir="$tmp_dir/rendered/winget"
installer_manifest="$manifest_dir/0state.scafld.installer.yaml"
if [[ ! -f "$installer_manifest" ]]; then
  echo "missing rendered WinGet installer manifest in release artifact" >&2
  exit 1
fi

sha_for() {
  local file="$1"
  local sha
  sha="$(awk -v f="$file" '$2 == f {print $1}' "$checksums_file")"
  if [[ -z "$sha" ]]; then
    echo "missing checksum for $file in release checksums.txt" >&2
    exit 1
  fi
  printf '%s' "$sha"
}

verify_manifest_hashes() {
  local url=""
  local expected=""
  local actual=""
  local checked=0

  while IFS= read -r line; do
    case "$line" in
      *InstallerUrl:*)
        url="${line#*InstallerUrl: }"
        ;;
      *InstallerSha256:*)
        actual="${line#*InstallerSha256: }"
        if [[ -z "$url" ]]; then
          echo "InstallerSha256 appeared before InstallerUrl" >&2
          exit 1
        fi
        expected="$(sha_for "${url##*/}")"
        if [[ "$actual" != "$expected" ]]; then
          echo "hash mismatch for ${url##*/}: manifest=$actual release=$expected" >&2
          exit 1
        fi
        checked=$((checked + 1))
        url=""
        ;;
    esac
  done < "$installer_manifest"

  if [[ "$checked" -eq 0 ]]; then
    echo "no installer hashes found in $installer_manifest" >&2
    exit 1
  fi
}

verify_manifest_hashes

target_dir="$winget_repo/manifests/0/0state/scafld/$version"
mkdir -p "$target_dir"
cp "$manifest_dir"/0state.scafld*.yaml "$target_dir/"

echo "Staged verified WinGet manifests in $target_dir"
echo "Next:"
echo "  cd $winget_repo"
echo "  git checkout -b scafld-$version"
echo "  git add manifests/0/0state/scafld/$version"
echo "  git commit -m 'Add 0state.scafld $version'"
