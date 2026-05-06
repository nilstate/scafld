package cli

import (
	"fmt"
	"io"
	"strings"
)

func knownCommand(name string) bool {
	return commandHandlers[name] != nil
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, "scafld - deterministic protocol for multi-phase agent work\n\nUsage:\n  scafld <command> [flags]\n\nCommands:\n")
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %-10s %s\n", cmd.name, cmd.summary)
	}
	fmt.Fprint(w, "\nFlags:\n  --root PATH    Workspace root\n  --json         Print JSON envelope\n  -h, --help     Show help\n  --version      Show version\n")
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
