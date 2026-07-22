# Threat Model

This document states what a passing `scafld verify` proves, what it assumes,
and what it cannot detect.

## What a Receipt Binds

A signed receipt binds the canonical receipt body to one trusted Ed25519 public
key. The signed body includes:

- task id and session id
- snapshot mode, base ref when present, base commit, and head commit
- tree SHA, in-scope file digests, reviewed provenance, and explicitly ignored
  unreviewed paths
- reviewer identity, host identity, independence classification, and downgrade
  reason when independence is below cross-vendor
- acceptance commands and observed results
- open blockers, mutation guard state, ledger head, and minted time

`scafld verify` checks that signature, recomputes the checkout fingerprint or
the explicit material-ref fingerprint, checks base ancestry against the explicit
protected-base target and verified material, re-runs signed acceptance commands,
and enforces configured minimum independence.

In pull-request CI, scafld's packaged `pull_request_target` workflow runs from
trusted base workflow code and splits that proof into two required lanes. The
material lane verifies signed receipt and fingerprint data without executing the
proposed worktree. The acceptance lane runs recorded commands from a clean
checkout at the same material commit with persisted checkout credentials
disabled and token-like CI environment variables scrubbed. `scafld verify
--material-only` is only the first lane; it is not a complete merge-wall proof
unless paired with a passing full acceptance lane and aggregate gate.

## Trusted Keys

`.scafld/trusted-keys.json` is the committed trust anchor. It is trust on first
commit, not an external PKI. A key entry may include `revoked`, `revoked_at`,
and `expires_at`.

Absent lifecycle fields keep historical behavior: the key remains active unless
`revoked` is true. When lifecycle fields are present, verification and session
ledger replay evaluate the key at the receipt's signed `minted_at` time:

- `revoked: true` rejects every receipt using that key.
- `revoked_at` rejects receipts minted at or after that timestamp.
- `expires_at` rejects receipts minted at or after that timestamp.

Old binaries that reject unknown trusted-key fields may not read lifecycle
metadata. Roll back by removing lifecycle fields or by using a binary that
understands the schema.

## What the Host Key Can Forge

The private signing key lives on the host. Anyone who can read or use that key
can mint a receipt body. Verification catches mismatches between the signed body
and the current checkout, target ancestry, acceptance reruns, key lifecycle, and
declared independence policy. It cannot prove the host key was never exposed.

Protect the host key as local signing material. `scafld verify --self-check`
reports the configured trusted keys and signing-key file permissions so
operators can see local hygiene issues before relying on receipts.

## Independence Limits

`cross_vendor` means scafld could derive distinct known model vendors for the
host and reviewer. `isolation_only` means the reviewer was isolated from the
host process, but distinct vendors were not established.

Independence detection is not organizational proof. It does not prove legal
separation, employment separation, account separation, or that a provider's
backend never shares training or routing infrastructure. It only proves the
runtime facts scafld can derive and signs those facts into the receipt.

## Ledger Integrity

The session ledger is append-only evidence. Receipt entries chain through a
ledger head, and appends are locked across processes on supported install
platforms. On Unix scafld uses `flock`; on Windows it uses `LockFileEx`.

Fast append and report paths may use persisted metadata, but full replay remains
the source of truth for loading and verification. If cached ledger metadata does
not cross-check, scafld falls back to full replay and fails closed if replay
cannot establish a valid chain.

## Out of Scope

scafld does not claim to:

- protect against a malicious maintainer who commits a malicious trusted key
- prove the host signing key was never copied
- prove legal or organizational independence
- enforce GitHub branch protection settings
- guarantee GitHub secrets are unavailable if a workflow author explicitly
  passes them into acceptance commands outside scafld's packaged verifier
- replace project-specific security review, secrets scanning, or deployment
  policy
