package reviewsubmit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	submitmcp "github.com/nilstate/scafld/v2/internal/adapters/mcp/reviewsubmit"
)

// ErrInvalid wraps invalid hidden-command arguments.
var ErrInvalid = errors.New("invalid review-submit-stdio arguments")

// Handler returns a CLI-compatible command handler for review-submit-stdio.
func Handler(stdin io.Reader) func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		if err := Run(ctx, args, stdin, stdout, stderr); err != nil {
			exit := 1
			if errors.Is(err, ErrInvalid) {
				exit = 2
			}
			fmt.Fprintf(stderr, "error: %v\n", err)
			return exit
		}
		return 0
	}
}

// Run starts the review submission MCP stdio server.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	outPath, err := parseOptions(args)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return submitmcp.Run(ctx, stdin, stdout, stderr, outPath)
}

func parseOptions(args []string) (string, error) {
	var outPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--out":
			if i+1 >= len(args) {
				return "", errors.New("--out requires a value")
			}
			outPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--out="):
			outPath = strings.TrimPrefix(arg, "--out=")
		default:
			return "", fmt.Errorf("unknown argument %q", arg)
		}
	}
	if strings.TrimSpace(outPath) == "" {
		return "", errors.New("--out is required")
	}
	return outPath, nil
}
