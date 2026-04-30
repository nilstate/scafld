# Releasing scafld

This is a maintainer doc. Most contributors do not need it.

## Pipeline

`.github/workflows/release.yml` fires on tags matching `v*.*.*`.

Stages:

1. **validate** — version sync, py_compile, review-gate / harden / update / package smokes.
2. **build** — produces wheel and sdist into `release-artifacts/dist`.
3. **publish-pypi** — uploads to PyPI via OIDC trusted publishing.
4. **publish-npm** — `npm publish` using the `NPM_TOKEN` secret.
5. **github-release** — creates the GitHub release with artifacts attached.

## PyPI trusted publishing

The `publish-pypi` job authenticates to PyPI without a long-lived token.
GitHub mints a short-lived OIDC token on every run; PyPI exchanges that
token for a one-shot upload credential scoped to this exact workflow and
job.

### One-time setup on PyPI

The `scafld` project already exists, so use the *Add a trusted publisher*
flow on the existing project page rather than the *pending publisher*
form (the latter is only for projects that have not been created yet).

1. Open the `scafld` project on PyPI → *Manage* → *Publishing* → *Add a
   trusted publisher* → *GitHub*.
2. Fill in:
   - **Owner**: `nilstate`
   - **Repository name**: `scafld`
   - **Workflow name**: `release.yml`
   - **Environment name**: leave empty
3. Save.

The job now runs with `permissions.id-token: write` and uploads to PyPI
on every tag push without referencing any repository secret.

### Verifying with TestPyPI before deleting the old token

Before removing `PYPI_API_TOKEN`, dry-run the binding against TestPyPI
to confirm the OIDC handshake works without a password. Do not use a
real tag for the dry-run — that fires the full release workflow and
would also publish to npm and create a GitHub release.

1. Add the same trusted publisher binding to the corresponding TestPyPI
   project (`https://test.pypi.org/manage/project/scafld/`).
2. On a throwaway branch, copy `release.yml` to `.github/workflows/
   trusted-publish-dryrun.yml`, strip every job except `validate`,
   `build`, and `publish-pypi`, change the `publish-pypi` step to
   include `repository-url: https://test.pypi.org/legacy/`, and trigger
   it via `workflow_dispatch` from the GitHub Actions tab.
3. Confirm the TestPyPI upload succeeds, then delete the dry-run
   workflow file. Do not merge it.

### Removing the old token

`PYPI_API_TOKEN` may still exist in the repository's *Settings → Secrets
and variables → Actions*. Leave it in place until both:

- the TestPyPI dry-run above has succeeded, and
- one real PyPI release has succeeded under trusted publishing.

Then delete the secret. The workflow no longer reads it.

### Rollback

If a release fails because of a misconfigured trusted publisher binding:

1. In `release.yml`, restore the previous step shape on `publish-pypi`:
   ```yaml
   - name: Publish to PyPI
     uses: pypa/gh-action-pypi-publish@release/v1
     with:
       packages-dir: release-artifacts/dist
       password: ${{ secrets.PYPI_API_TOKEN }}
       skip-existing: true
   ```
2. Remove the `id-token: write` permission line.
3. Re-create the `PYPI_API_TOKEN` secret if it was deleted.
4. Re-tag and push.

## npm publishing

Still uses the `NPM_TOKEN` repository secret. npm's OIDC and provenance
story is not adopted here; revisit when it stabilises across the npm
registry, GitHub Actions, and supply-chain consumers.

## Cutting a release

```bash
python3 scripts/bump_version.py X.Y.Z
git add -A
git commit -m "release: vX.Y.Z"
git tag vX.Y.Z
git push origin main vX.Y.Z
```

The workflow runs end-to-end on the tag push. Watch the *Actions* tab.
