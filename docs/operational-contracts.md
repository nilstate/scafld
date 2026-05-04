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
