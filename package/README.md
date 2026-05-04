# Package Adapters

The Go binary is the product. Everything in this directory is a package-manager
adapter that installs or points at a GitHub release asset.

## Owned adapters

- `npm/` publishes the `scafld` npm package.
- `pypi/` publishes the `scafld` PyPI package.

## Registry templates

- `homebrew/` is for `nilstate/homebrew-tap`.
- `scoop/` is for `nilstate/scoop-bucket`.
- `winget/` is for WinGet submissions.
- `docker/` documents the OCI image shape.

Adapters must not duplicate scafld behavior. They may only download, verify,
cache, install, or execute the native binary produced by
`scripts/build-release-artifacts.sh`.
