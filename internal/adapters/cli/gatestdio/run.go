package gatestdio

import (
	"context"
	"fmt"
	"io"

	hostgate "github.com/nilstate/scafld/v2/internal/adapters/mcp/hostgate"
)

// Handler returns a CLI-compatible MCP stdio server handler for scafld_gate.
func Handler(binary string, stdin io.Reader) func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		if len(args) != 0 {
			fmt.Fprintf(stderr, "error: gate-stdio accepts no arguments\n")
			return 2
		}
		if err := hostgate.Run(ctx, stdin, stdout, stderr, hostgate.Options{ScafldBinary: binary}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
}
