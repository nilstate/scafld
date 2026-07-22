# scafld Agent Contract

## Default Agent Flow

Work with the host agent's normal planning, editing, and testing tools. When the work appears done, call `finalize`.

`finalize` is the accountability surface: it records acceptance evidence, runs the independent review path, and returns either blockers or a signed receipt. The agent does not grade its own completion.

The receipt reports its independence level honestly. `cross_vendor` means multi-model review that can reduce correlated blind spots; it is still single-party local tooling unless a separate operator or CI trust domain verifies the receipt. `isolation_only` means the reviewer was isolated but cross-vendor separation was not proven.

## Merge Wall

CI runs `scafld verify <receipt> --target <commit-ish>` against the signed receipt. This is the hard wall for merging. The Claude Stop hook is only a local affordance; it can be bypassed in subagents, piped runs, Codex, Gemini, or other hosts.

## Human And CI Lifecycle

The full CLI lifecycle remains available for operators, automation, and debugging:

```bash
scafld init
scafld plan <task-id> --title "Title" --size small --risk low
scafld harden <task-id>
scafld validate <task-id>
scafld approve <task-id>
scafld build <task-id>
scafld review <task-id>
scafld complete <task-id>
scafld verify <receipt> --target <commit-ish>
scafld status <task-id>
scafld handoff <task-id>
```

Use `scafld harden` to strengthen drafts before approval. Use `scafld build` to record phase evidence. Use `scafld review` when running the lifecycle directly. Use `scafld status --json` for automation.

If harden evidence is incomplete, stale, failed, or `needs_revision`, approval
requires `scafld approve <task-id> --reason <reason>`. Fix real shape blockers
in the draft and rerun harden; use a reason only for an explicit operator
decision to reject bookkeeping, advisory, or overengineering findings.

## Agent Context Hierarchy

Use structured JSON for lifecycle state and gate state. Use the embedded
`Source Spec Markdown` section as the canonical task contract when it is present
in a scafld packet. Derived sections are projections and indexes over those
sources, not independent contracts.

## Do Not

- Close governed work without `finalize` or an explicit human decision.
- Reconstruct lifecycle state by scraping Markdown. Use `status --json`.
- Act from an older scafld packet when a newer status, handoff, harden, or
  review packet is available.
- Mutate `.scafld/core/` by hand. Use `scafld update`.
- Treat the Stop hook as the merge wall. CI verify is the wall.
- Cite files, commands, receipts, or review findings you have not verified.

## Prompts

`.scafld/prompts/*` overrides `.scafld/core/prompts/*` overrides built-ins.

# Source Checkout

Inside the scafld repo, use the machine-default `scafld` only when it resolves to
this checkout's `bin/scafld` dev launcher, or call `./bin/scafld` directly. Do
not use a copied compiled binary; stale binaries can report old lifecycle state.
