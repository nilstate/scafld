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
  --provider NAME          Review provider: auto, codex, claude, command, or local
  --model MODEL            Provider model override
  --provider-binary PATH   Provider binary override
  --provider-command CMD   Command provider shell command
  --review-scope PATHS     Comma-separated paths that override derived task scope
  --root PATH              Workspace root
  --json                   Print JSON envelope
  -h, --help               Show help

Review scope:
  scafld derives review scope from spec packages, impacted files, and phase
  changes. Use --review-scope only when a dirty monorepo needs an explicit
  path boundary:

    scafld review email-contracts --review-scope api
    scafld review email-contracts --review-scope api,cli/packages/mcp

  The approval baseline is recorded before task execution. Unchanged baseline
  dirt is context, not a finding by itself. Files changed during review still
  fail closed.
`)
}
