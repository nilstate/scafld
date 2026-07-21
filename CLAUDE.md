# scafld Claude Contract

Read `AGENTS.md` first. It owns the full scafld contract.

## Default Flow

Use the `/finalize` slash command or call the `finalize` MCP tool when work is ready. Report blockers if the independent finalize fails. Report the signed receipt when it passes.

## Boundaries

- Work normally before the finalize; do not hand-sequence the lifecycle for routine agent work.
- Use `finalize` as the default adversarial finalize.
- Use `scafld verify <receipt> --target <commit-ish>` in CI as the hard merge wall.
- Treat the Stop hook as a local Claude Code affordance only; it is not a CI guarantee and does not cover subagents or other hosts.
- Treat `cross_vendor` as multi-model, single-party evidence. It reduces correlated blind spots, but it is not a separate human or organizational trust boundary.
- Use `scafld status --json` for automation.

For direct lifecycle work, real review uses `--provider claude`, `--provider codex`, or `--provider gemini`.
`--provider local` is smoke-test only and cannot satisfy `complete`.
