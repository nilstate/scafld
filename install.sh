#!/usr/bin/env sh
set -eu

REPO="${SCAFLD_GITHUB_REPOSITORY:-nilstate/scafld}"
VERSION="${VERSION:-}"
BIN_DIR="${BIN_DIR:-$HOME/.local/bin}"

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"

case "$os" in
  darwin) goos="darwin" ;;
  linux) goos="linux" ;;
  *) echo "unsupported OS: $os" >&2; exit 2 ;;
esac

case "$arch" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported architecture: $arch" >&2; exit 2 ;;
esac

if [ -z "$VERSION" ]; then
  VERSION="$(
    curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" |
      sed -n 's/.*"tag_name": "v\([^"]*\)".*/\1/p' |
      head -n 1
  )"
fi

if [ -z "$VERSION" ]; then
  echo "could not resolve latest scafld release version" >&2
  exit 1
fi

asset="scafld_${VERSION}_${goos}_${goarch}"
base="https://github.com/$REPO/releases/download/v$VERSION"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT INT TERM

echo "Installing scafld v$VERSION for $goos/$goarch..."
curl -fsSL "$base/checksums.txt" -o "$tmpdir/checksums.txt"
curl -fsSL "$base/$asset" -o "$tmpdir/scafld"

expected="$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1 }' "$tmpdir/checksums.txt")"
if [ -z "$expected" ]; then
  echo "checksums.txt does not contain $asset" >&2
  exit 1
fi

actual="$(shasum -a 256 "$tmpdir/scafld" | awk '{ print $1 }')"
if [ "$actual" != "$expected" ]; then
  echo "checksum mismatch for $asset" >&2
  echo "expected: $expected" >&2
  echo "actual:   $actual" >&2
  exit 1
fi

mkdir -p "$BIN_DIR"
install -m 0755 "$tmpdir/scafld" "$BIN_DIR/scafld"
echo "Installed scafld -> $BIN_DIR/scafld"

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo ""
    echo "Add $BIN_DIR to your PATH:"
    echo "  export PATH=\"$BIN_DIR:\$PATH\""
    ;;
esac
