package finalizestdio

import (
	"context"
	"fmt"
	"io"

	finalizemcp "github.com/nilstate/scafld/v2/internal/adapters/mcp/finalize"
)

// Handler returns a CLI-compatible MCP stdio server handler for finalize.
func Handler(binary string, stdin io.Reader) func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		if len(args) != 0 {
			fmt.Fprintf(stderr, "error: finalize-stdio accepts no arguments\n")
			return 2
		}
		if err := finalizemcp.Run(ctx, stdin, stdout, stderr, finalizemcp.Options{ScafldBinary: binary}); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		return 0
	}
}
