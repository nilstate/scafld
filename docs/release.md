# Releasing scafld

This is a maintainer doc. Most contributors do not need it.

## Identity

The primary repository is `github.com/nilstate/scafld`. The Go module path is
`github.com/nilstate/scafld/v2` because the product release line is already v2.

Release tags are `vX.Y.Z`. The first Go-backed release is `v2.1.0`.

## Pipeline

`.github/workflows/release.yml` fires on tags matching `v*.*.*`.

Stages:

1. **validate** -- run `make check` and build a release snapshot.
2. **github-release** -- build native Go binaries, `checksums.txt`, and
   `manifest.json`, then create the GitHub release.
3. **publish-pypi** -- build and publish the PyPI launcher wrapper.
4. **publish-npm** -- publish the npm launcher wrapper.

The order is intentional: npm and PyPI wrappers download and verify GitHub
release assets, so the GitHub release must exist before either wrapper package
is published.

## Versioning

Use the same product version everywhere:

- GitHub tag: `v2.1.0`
- npm: `scafld@2.1.0`
- PyPI: `scafld==2.1.0`
- Go module install: `go install github.com/nilstate/scafld/v2/cmd/scafld@v2.1.0`

Before a release, run:

```bash
scripts/set-release-version.sh 2.1.0
make check
make release-snapshot
```

## Registry Publishing

The workflow currently reuses the existing repository secrets:

- `PYPI_API_TOKEN`
- `NPM_TOKEN`

The wrappers are thin launchers. They never reimplement scafld behavior; they
download the matching native binary and verify it against `checksums.txt`.

## Cutting a Release

```bash
scripts/set-release-version.sh X.Y.Z
git add -A
git commit -m "release: vX.Y.Z"
git tag vX.Y.Z
git push origin main vX.Y.Z
```

Watch the Actions tab. Verify:

- GitHub release contains all six native binaries plus `checksums.txt` and
  `manifest.json`.
- `npm install -g scafld@X.Y.Z` works.
- `pipx install scafld==X.Y.Z` works.
- `go install github.com/nilstate/scafld/v2/cmd/scafld@vX.Y.Z` works.
