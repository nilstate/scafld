package review

import (
	"fmt"
	"io"
)

// PrintHelp writes command-specific review help.
func PrintHelp(w io.Writer) {
	fmt.Fprint(w, `scafld review - Run the adversarial review gate

Usage:
  scafld review <task_id> [flags]

Flags:
  --provider NAME          Review provider: auto, codex, claude, gemini, command, or local
  --model MODEL            Provider model override
  --provider-binary PATH   Provider binary override
  --provider-command CMD   Command provider shell command
  --review-scope PATHS     Comma-separated paths that override derived task scope
  --mode MODE              Review mode: discover or verify
  --max-findings N         Bound provider output volume
  --min-attack-angles N    Request at least N attack-log entries
  --review-depth TEXT      Review depth: light, standard, or deep
  --force                  Rerun even when the current review already passed
  --print-context          Print the exact provider context and exit
  --human-reviewed         Record an audited human review instead of invoking a provider
  --reason TEXT            Required reason for --human-reviewed
  --root PATH              Workspace root
  --json                   Print JSON envelope
  -h, --help               Show help

Review scope:
  scafld derives review scope from spec packages, impacted files, and phase
  changes. Use --review-scope only when a dirty monorepo needs an explicit
  path boundary:

    scafld review email-contracts --review-scope api
    scafld review email-contracts --review-scope api,cli/packages/mcp

  The approval baseline is recorded before task execution. Outside-scope drift
  is included as ambient workspace context, not blocked before provider spend.
  Task-relevant changes during review still fail closed.

Provider auto:
  auto prefers the other installed agent when scafld can infer the current
  host agent, can use Gemini as an additional external challenger, and fails
  closed when only the host provider is available unless fallback_policy is
  relaxed or a provider is selected explicitly. Set
  SCAFLD_HOST_AGENT=codex or SCAFLD_HOST_AGENT=claude when a wrapper does not
  expose a recognizable host marker.

Context:
  Print the deterministic reviewer brief without spending provider tokens:
    scafld review email-contracts --print-context

Command provider contract:
  --provider command runs --provider-command and expects one complete
  ReviewDossier JSON object on stdout. Progress and diagnostics belong on
  stderr; a non-zero exit or malformed stdout fails the review attempt.

Cost control:
  For small diffs, keep the same gate but request a tighter review budget:
    scafld review email-contracts --review-depth light --max-findings 4 --min-attack-angles 3

Human review:
  Use --human-reviewed only after operator review; --reason records the audit reason:
    scafld review email-contracts --human-reviewed --reason "reviewed PR 123"
`)
}
