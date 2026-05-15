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
	fmt.Fprint(w, "\nFlags:\n  --root PATH    Workspace root\n  --json         Print JSON envelope\n  -h, --help     Show help\n  --version      Show version\n")
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
  scafld harden <task_id> [--mark-passed] [--root PATH] [--json]

Without flags, opens a harden round and prints the active prompt. The agent
attacks the draft contract before build.

Required checks:
  Path audit
  Command audit
  Scope/migration audit
  Acceptance timing audit
  Rollback/repair audit
  Design challenge

Each check needs Grounded in, Result, and Evidence. Result must be passed or
not_applicable before --mark-passed can close the round.
Design challenge must question why the plan exists, whether it solves the
underlying problem, and whether it is a short-sighted bandaid or future bloat.
Grounded in accepts spec_gap:<section>, archive:<task-id>, or a single-line
code citation such as code:src/file.go:42. Line ranges are rejected.

Questions are optional. Questions: none is valid only after the checks have
evidence. Any recorded question needs a recommended answer and final answer.

Flags:
  --mark-passed  Verify harden evidence and close the latest round
  --root PATH    Workspace root
  --json         Print JSON envelope
`)
		return
	}
	if name == "update" {
		fmt.Fprint(w, `scafld update - Refresh managed scafld files

Usage:
  scafld update [--root PATH] [--json]

Refreshes .scafld/core, default project prompt copies, root agent docs, and
renders generated project config into the current strict runtime shape.

Use this after upgrading scafld or when config parsing tells you to run update.
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
