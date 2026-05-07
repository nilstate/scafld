# Package Adapters

The Go binary is the product. Everything in this directory is a package-manager
adapter that installs or points at a GitHub release asset.

## Owned adapters

- `npm/` publishes the `scafld` npm package.
- `pypi/` publishes the `scafld` PyPI package.
- `homebrew/` renders and publishes the `nilstate/homebrew-tap` formula.
- `scoop/` renders and publishes the `nilstate/scoop-bucket` manifest.
- `docker/` publishes the GHCR OCI image.

## Registry templates

- `winget/` renders manifests for WinGet submissions.

Adapters must not duplicate scafld behavior. They may only download, verify,
cache, install, or execute the native binary produced by
`scripts/build-release-artifacts.sh`.

The tag release workflow renders package-manager manifests from these templates.
Channels owned by 0state publish automatically when their repository tokens are
configured. WinGet manifests are uploaded as rendered release artifacts for the
Microsoft submission flow.
