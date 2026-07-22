// Package help formats command help for the CLI adapter.
package help

import (
	"fmt"
	"io"

	reviewhelp "github.com/nilstate/scafld/v2/internal/adapters/cli/review"
)

// Command is one command row in the root help output.
type Command struct {
	Name    string
	Summary string
}

// Print writes root command help.
func Print(w io.Writer, commands []Command) {
	fmt.Fprint(w, "scafld - deterministic protocol for multi-phase agent work\n\nUsage:\n  scafld <command> [flags]\n\nCommands:\n")
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", cmd.Name, cmd.Summary)
	}
	fmt.Fprint(w, "\nFlags:\n  --root PATH    Workspace root\n  --json         Print JSON envelope\n  --no-context   Omit source markdown where supported while keeping provenance\n  -h, --help     Show help\n  --version      Show version\n")
}

// PrintCommand writes command-specific help.
func PrintCommand(w io.Writer, name string, commands []Command) {
	if name == "review" {
		reviewhelp.PrintHelp(w)
		return
	}
	if name == "harden" {
		fmt.Fprint(w, `scafld harden - Stress-test a draft spec before approval

Usage:
  scafld harden <task_id> [--provider auto|codex|claude|gemini|command|local] [--root PATH] [--json]
  scafld harden <task_id> --mark-passed [--root PATH] [--json]

Without flags, opens a harden round and prints the source-backed harden context
by default. The embedded Source Spec Markdown section is the canonical draft
contract; derived sections are indexes only. Use --no-context only for scripts
that intentionally need terse output.

With --provider, scafld delegates hardening to a separate read-only provider.
The provider must submit one HardenDossier through the structured channel.
scafld derives pass or needs_revision from the submitted shape decision,
required spec edits, and observations. Provider transport, invalid dossier, or
unverified anchor problems are recorded as harden_status error. Advisory
observations remain recorded.
Provider auto prefers the other installed agent when the host is detected, can
use Gemini as an additional external challenger, and fails closed when only the
host provider is available unless fallback_policy is relaxed or a provider is
selected explicitly.

Required observation dimensions:
  design
  scope
  path
  command
  timing
  rollback

Each harden round needs a shape decision: keep, shrink, reframe, or reject. A
passing round must use keep and leave Required spec edits empty.

Harden is draft-revision bound. Re-running harden while a round is in progress
prints that same round instead of appending another. A provider needs_revision
round blocks another provider pass against the same draft, but not operator
approval: revise true shape blockers, or run approve with --reason when the
finding is rejected as bookkeeping/advisory/overengineering. Advisories stay
recorded and do not force a new round.

Each observation needs Result and Anchor. Result must be clean, advisory, blocks,
or n/a. Open blocks keep the round from passing until fixed, accepted_risk, or
superseded. The design dimension must question why the plan exists, which
shared core/app contract owns the behavior, and whether API/MCP/CLI/provider/docs
surfaces stay light adapters instead of separate implementations.
Anchor accepts spec_gap:<section>, archive:<task-id>, or a single-line code
citation such as code:src/file.go:42. Line ranges are rejected.
Advisory observations keep full detail but do not block.

Flags:
  --provider NAME       Run provider-backed hardening
  --provider-command C  Command provider executable or shell command
  --provider-binary P   Selected provider binary override
  --model NAME          Selected provider model override
  --mark-passed         Verify manual harden evidence and close the latest round
  --no-context          Suppress source context in human/JSON output
  --root PATH           Workspace root
  --json                Print JSON envelope
`)
		return
	}
	if name == "approve" {
		fmt.Fprint(w, `scafld approve - Approve a draft spec

Usage:
  scafld approve <task_id> [--reason TEXT] [--root PATH] [--json]

Moves a draft spec into approved work and records an approval receipt. If harden
evidence is incomplete, stale, failed, or needs operator judgment, --reason is
required. scafld records a harden_override receipt and marks the harden state as
overridden before approval.

Use --reason to explain the operator decision, not to hide findings. Real shape
blockers should be fixed in the draft and hardened again; bookkeeping,
advisory, or overengineering findings can be rejected with a clear reason.

Flags:
  --reason TEXT  Operator rationale when approving over harden evidence
  --root PATH    Workspace root
  --json         Print JSON envelope
`)
		return
	}
	if name == "status" {
		fmt.Fprint(w, `scafld status - Show task status and next action

Usage:
  scafld status <task_id> [--root PATH] [--json] [--no-context]

Shows lifecycle status, gate state, latest review findings, task material, and
the deterministic next action. Agent entry should read full status or handoff
first so the source-backed contract is in context. Follow-up polling may use
--no-context; scafld still includes spec_source path, sha256, and byte count so
the agent can reload full context when the digest changes.

Flags:
  --no-context  Omit source markdown but keep spec_source provenance
  --root PATH   Workspace root
  --json        Print JSON envelope
`)
		return
	}
	if name == "verify" {
		fmt.Fprint(w, `scafld verify - Verify a signed scafld receipt

Usage:
  scafld verify <receipt-path> --target <commit-ish> [--material-ref <commit-ish>] [--acceptance-root PATH] [--trusted-keys PATH] [--ci] [--self-check] [--root PATH] [--json]
  scafld verify <receipt-path> --target <commit-ish> --material-ref <commit-ish> --material-only [--trusted-keys PATH] [--ci] [--root PATH]

Full verify checks signature, trusted keys, material digests, target/material
ancestry, independence, and recorded acceptance reruns.

When --material-ref is supplied, full verify runs acceptance only against a
clean checkout at that same commit. Use --acceptance-root for a detached
pull-request head worktree; omitting it is accepted only when --root already is
that clean material commit.

--material-only is for a separate CI material lane. It verifies the signed
receipt and material fingerprint but does not execute acceptance and is not a
complete merge gate by itself.

Flags:
  --target REF       Protected-base commit or ref
  --material-ref REF Verified material commit or ref
  --material-only    Verify signature/material only; skip acceptance
  --acceptance-root  Clean checkout used for acceptance reruns
  --trusted-keys     Trusted keys file
  --ci               Fail closed on missing CI policy inputs
  --self-check       Print local verify wiring report
  --root PATH        Workspace root
  --json             Print JSON envelope
`)
		return
	}
	if name == "update" {
		fmt.Fprint(w, `scafld update - Refresh managed scafld files

Usage:
  scafld update [--root PATH] [--json]

Refreshes .scafld/core, existing manifest-backed prompt copies, root agent docs,
and managed core assets. Project config is left untouched.

Use this after upgrading scafld.
`)
		return
	}
	if name == "adapter" {
		fmt.Fprint(w, `scafld adapter - Render provider-facing trigger packet

Usage:
  scafld adapter codex build <task_id> [--root PATH] [--json]
  scafld adapter claude review <task_id> [--root PATH] [--json]

The adapter command renders current status, deterministic next-action fields,
and the scafld handoff for external trigger wrappers. Source context is included
by default. Use --no-context only after the same spec_source sha256 was already
read; status keeps provenance while handoff suppresses the source markdown body.
It does not execute an agent runtime and does not advance lifecycle state.
`)
		return
	}
	for _, cmd := range commands {
		if cmd.Name == name {
			fmt.Fprintf(w, "scafld %s - %s\n\nUsage:\n  scafld %s [task_id] [flags]\n", cmd.Name, cmd.Summary, cmd.Name)
			return
		}
	}
	fmt.Fprintf(w, "scafld %s\n", name)
}
