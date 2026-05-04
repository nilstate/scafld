# Distribution

scafld is a Go CLI distributed through several package ecosystems. The Go
binary is authoritative; npm and PyPI packages are thin launchers that download
and verify the matching GitHub release asset.

## Primary channels

- Go modules: `go install github.com/nilstate/scafld/v2/cmd/scafld@latest`
- GitHub Releases: raw platform binaries plus `checksums.txt` and `manifest.json`
- Homebrew: `brew install nilstate/tap/scafld`
- npm: `npm install -g scafld`
- PyPI: `pipx install scafld`
- Scoop: `scoop bucket add nilstate https://github.com/nilstate/scoop-bucket`
  then `scoop install scafld`

## Secondary channels

These are generated from the GitHub release assets, not rebuilt from source in
separate registry flows:

- WinGet manifest submission: `0state.scafld`
- Docker/OCI image for CI runners
- Debian/RPM/AUR/Nix packages based on release binaries and checksums

Templates for Homebrew, Scoop, WinGet, and OCI live under `package/`. Homebrew
and Scoop render into owned registry repositories; WinGet requires upstream
review in `microsoft/winget-pkgs`.

## Release Contract

1. A tag `vX.Y.Z` is pushed in `github.com/nilstate/scafld`.
2. CI runs `make check`.
3. `scripts/build-release-artifacts.sh X.Y.Z` builds raw native binaries for
   Linux, macOS, and Windows on amd64/arm64.
4. GitHub release assets are published before npm/PyPI because those wrappers
   download and verify binaries from the release by version.
5. npm and PyPI publish the wrapper packages at the same product version.

Package wrappers must never reimplement scafld behavior. They may only locate,
download, verify, cache, install, and execute the native binary.
