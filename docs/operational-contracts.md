# Operational Contracts

Every IO port accepts context.Context. Use cases do not start work without one.

Errors are wrapped with `fmt.Errorf("%w", err)` and matched with `errors.Is` or
`errors.As`. Sentinel errors live in the owning package. CLI error codes are
mapped once in the CLI adapter.

## CLI exit code table

- `0`: success
- `1`: generic runtime failure
- `2`: invalid input or invalid spec
- `3`: validation or acceptance failure
- `4`: review gate failure
- `5`: cancelled or interrupted
- `6`: workspace or configuration failure

SIGINT and SIGTERM cancel the root context. Repeated interrupts escalate to
process termination after diagnostics have been recorded.

Workspace discovery is explicit: `--root` wins, then `SCAFLD_ROOT`, then cwd
walk-up until `.scafld/` is found.

## Ledger writes

Session ledger writes are read-modify-write operations and must hold both the
in-process path mutex and the cross-process lock file. Supported install
platforms use operating-system locks: Unix uses `flock`, and Windows uses
`LockFileEx`. A losing concurrent writer fails closed instead of overwriting or
silently dropping another receipt entry.

Full session replay remains the integrity authority. Fast append and reporting
paths may use persisted ledger metadata only after cross-checking the cached
ledger head; on mismatch they fall back to full replay and preserve the
fail-closed behavior.

## Receipt trust

Trusted-key lifecycle is evaluated at receipt `minted_at`, not at wall-clock
verification time. Missing `revoked_at` and `expires_at` fields keep historical
behavior, while present fields are enforced by verify and ledger replay through
a core-owned port.
