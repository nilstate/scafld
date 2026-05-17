# Releasing scafld

This is a maintainer doc. Most contributors do not need it.

## Identity

The primary repository is `github.com/nilstate/scafld`. The Go module path is
`github.com/nilstate/scafld/v2` because the product release line is already v2.

Release tags are `vX.Y.Z`. The source tree keeps wrapper package metadata at
the development placeholder version; release jobs stamp npm and PyPI metadata
from the tag.

## Pipeline

`.github/workflows/release.yml` fires on tags matching `v*.*.*`.

Stages:

1. **validate** -- run `make check` and build a release snapshot.
2. **github-release** -- build native Go binaries, `checksums.txt`, and
   `manifest.json`, then create the GitHub release.
3. **publish-pypi** -- build and publish the PyPI launcher wrapper.
4. **publish-npm** -- publish the npm launcher wrapper.
5. **package-managers** -- render Homebrew, Scoop, WinGet, and OCI manifests
   from the published release checksums and attach them to the release.
6. **publish-homebrew** and **publish-scoop** -- publish owned registry
   manifests when repository credentials are configured.
7. **publish-oci** -- publish the CI runner image to GHCR.

The order is intentional: npm and PyPI wrappers download and verify GitHub
release assets, so the GitHub release must exist before either wrapper package
is published.

## Versioning

Use the same product version everywhere in published artifacts:

- GitHub tag: `vX.Y.Z`
- npm: `scafld@X.Y.Z`
- PyPI: `scafld==X.Y.Z`
- Go module install: `go install github.com/nilstate/scafld/v2/cmd/scafld@vX.Y.Z`

Before a release, run:

```bash
make check
scripts/build-release-artifacts.sh X.Y.Z
```

Do not commit generated wrapper version changes. `.github/workflows/release.yml`
runs `scripts/set-release-version.sh "${GITHUB_REF_NAME#v}"` inside the release
job before building npm and PyPI packages.

## Registry Publishing

The workflow currently reuses the existing repository secrets:

- `PYPI_API_TOKEN`
- `NPM_TOKEN`

The wrappers are thin launchers. They never reimplement scafld behavior; they
download the matching native binary and verify it against `checksums.txt`.

## Cutting a Release

```bash
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
- `docker manifest inspect ghcr.io/nilstate/scafld:vX.Y.Z` works without
  authenticated package access.

## External Registry Follow-Up

After the GitHub release is live, publish or submit the manifests rendered from
the release checksums:

- Homebrew: update `nilstate/homebrew-tap` with `Formula/scafld.rb`.
- Scoop: update `nilstate/scoop-bucket` with `bucket/scafld.json`.
- WinGet: submit `0state.scafld` manifests to `microsoft/winget-pkgs`.
- GHCR: confirm the published image is publicly inspectable. If anonymous
  `docker manifest inspect ghcr.io/nilstate/scafld:vX.Y.Z` returns `denied`,
  make the `nilstate/scafld` container package public in GitHub Packages and
  rerun the metadata audit.

For WinGet, do not copy from a local `.stage/` directory. Stage the upstream
submission from the uploaded release artifact so the manifest hashes are checked
against the published `checksums.txt`:

```bash
scripts/prepare-winget-submission.sh X.Y.Z /path/to/winget-pkgs
```

That command downloads `package-managers-rendered.tar.gz` and `checksums.txt`
from the GitHub release, verifies every `InstallerSha256`, and copies the
versioned manifests into the WinGet checkout.
