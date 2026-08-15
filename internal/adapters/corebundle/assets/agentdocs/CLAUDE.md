# scafld Claude Contract

Read `AGENTS.md` first. It owns the full scafld contract.

## Default Flow

Run the review gate when work is ready, then use the `/finalize` slash command or `finalize` MCP tool after review passes. Report review blockers or the signed receipt from deterministic finalization.

## Boundaries

- Work normally before the review; review is the only provider/model call.
- Use `finalize` after a passing review as the deterministic final gate; it does not invoke a model.
- Use `scafld verify <receipt> --target <commit-ish>` in CI as the hard merge wall.
- Treat the Stop hook as a local Claude Code affordance only; it is not a CI guarantee and does not cover subagents or other hosts.
- Treat `cross_vendor` as multi-model, single-party evidence. It reduces correlated blind spots, but it is not a separate human or organizational trust boundary.
- Use `scafld status --json` for automation.

For a manual acceptance criterion, use the printed `scafld build <task-id>
--criterion <id> --disposition pass --evidence-digest <sha256> --actor <actor>
--reason <what-was-verified>` command. That one build invocation records the
verified evidence and re-evaluates acceptance. Do not edit criterion state or
replace a manual check with a fake shell command.

For direct lifecycle work, real review uses `--provider claude`, `--provider codex`, or `--provider gemini`.
`--provider local` is smoke-test only and cannot satisfy `finalize`.
