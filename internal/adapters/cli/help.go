package cli

import (
	"fmt"
	"io"
	"strings"
)

func knownCommand(name string) bool {
	for _, cmd := range commands {
		if cmd.name == name {
			return true
		}
	}
	return false
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "scafld - evidence-backed AI coding workflow")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  scafld <command> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", cmd.name, cmd.summary)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, "  --root PATH    Workspace root")
	fmt.Fprintln(w, "  --json         Print JSON envelope")
	fmt.Fprintln(w, "  -h, --help     Show help")
	fmt.Fprintln(w, "  --version      Show version")
}

func printCommandHelp(w io.Writer, name string) {
	for _, cmd := range commands {
		if cmd.name == name {
			fmt.Fprintf(w, "scafld %s - %s\n", cmd.name, cmd.summary)
			fmt.Fprintln(w)
			fmt.Fprintf(w, "Usage:\n  scafld %s [task_id] [flags]\n", cmd.name)
			return
		}
	}
	fmt.Fprintf(w, "scafld %s\n", strings.TrimSpace(name))
}
