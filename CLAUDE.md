# scafld Claude Contract

Read `AGENTS.md` first. It owns the full scafld contract.

## Default Flow

Use the `/scafld-gate` slash command or call the `scafld_gate` MCP tool when work is ready. Report blockers if the independent gate fails. Report the signed receipt when it passes.

## Boundaries

- Work normally before the gate; do not hand-sequence the lifecycle for routine agent work.
- Use `scafld_gate` as the default adversarial gate.
- Use `scafld verify <receipt> --target <commit-ish>` in CI as the hard merge wall.
- Treat the Stop hook as a local Claude Code affordance only; it is not a CI guarantee and does not cover subagents or other hosts.
- Use `scafld status --json` for automation.

For direct lifecycle work, real review uses `--provider claude`, `--provider codex`, or `--provider gemini`.
`--provider local` is smoke-test only and cannot satisfy `complete`.

Inside the scafld repo, use `./bin/scafld` or `go run ./cmd/scafld`; do not use a copied compiled binary.
