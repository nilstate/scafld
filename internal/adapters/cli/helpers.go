package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nilstate/scafld/v2/internal/adapters/clock"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/app/cancel"
	"github.com/nilstate/scafld/v2/internal/app/complete"
	"github.com/nilstate/scafld/v2/internal/app/contracts"
	"github.com/nilstate/scafld/v2/internal/app/fail"
)

type options struct {
	Root        string
	JSON        bool
	Values      map[string]string
	Positionals []string
}

func parseOptions(args []string) (options, error) {
	opts := options{Values: map[string]string{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--json" {
			opts.JSON = true
			continue
		}
		if arg == "--root" || arg == "--title" || arg == "--summary" || arg == "--command" || arg == "--size" || arg == "--risk" || arg == "--reason" || arg == "--provider" || arg == "--provider-command" || arg == "--provider-binary" || arg == "--model" {
			if i+1 >= len(args) {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			key := strings.TrimPrefix(arg, "--")
			if key == "root" {
				opts.Root = args[i+1]
			} else {
				opts.Values[key] = args[i+1]
			}
			i++
			continue
		}
		if strings.HasPrefix(arg, "--root=") {
			opts.Root = strings.TrimPrefix(arg, "--root=")
			continue
		}
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			key, value, _ := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
			opts.Values[key] = value
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unknown flag %q", arg)
		}
		opts.Positionals = append(opts.Positionals, arg)
	}
	return opts, nil
}

func normalizeGlobalFlags(args []string) []string {
	var globals []string
	i := 0
	for i < len(args) {
		switch {
		case args[i] == "--json":
			globals = append(globals, args[i])
			i++
		case args[i] == "--root" && i+1 < len(args):
			globals = append(globals, args[i], args[i+1])
			i += 2
		case strings.HasPrefix(args[i], "--root="):
			globals = append(globals, args[i])
			i++
		default:
			if len(globals) == 0 {
				return args
			}
			next := append([]string{args[i]}, args[i+1:]...)
			return append(next, globals...)
		}
	}
	return args
}

func oneTask(args []string, command string) (options, error) {
	opts, err := parseOptions(args)
	if err != nil {
		return opts, err
	}
	if len(opts.Positionals) != 1 {
		return opts, fmt.Errorf("%s requires task_id", command)
	}
	return opts, nil
}

func commandRoot(ctx context.Context, opts options, creating bool) (string, error) {
	if creating && opts.Root == "" {
		return filesystem.ResolveRoot(ctx, ".", filesystem.Discovery{})
	}
	return filesystem.ResolveRoot(ctx, opts.Root, filesystem.Discovery{})
}

func stores(ctx context.Context, opts options) (markdown.Store, jsonstore.SessionStore, int, error) {
	root, err := commandRoot(ctx, opts, false)
	if err != nil {
		return markdown.Store{}, jsonstore.SessionStore{}, ExitWorkspace, err
	}
	return markdown.Store{Root: root}, jsonstore.SessionStore{Root: root}, ExitSuccess, nil
}

func okOut[T any](w io.Writer, command string, result T, text string, asJSON bool, code ...int) int {
	exit := ExitSuccess
	if len(code) > 0 {
		exit = code[0]
	}
	if asJSON {
		_ = json.NewEncoder(w).Encode(contracts.Envelope[T]{OK: exit == ExitSuccess, Command: command, Result: result})
		return exit
	}
	fmt.Fprint(w, text)
	return exit
}

func failOut(w io.Writer, err error, exit int, asJSON bool) int {
	if err == nil {
		err = errors.New("unknown error")
	}
	if errors.Is(err, context.Canceled) {
		exit = ExitCancelled
	}
	if asJSON {
		_ = json.NewEncoder(w).Encode(contracts.Envelope[map[string]any]{
			OK: false,
			Error: &contracts.Error{
				Code:     codeName(exit),
				Message:  err.Error(),
				ExitCode: exit,
			},
		})
		return exit
	}
	fmt.Fprintf(w, "error: %v\n", err)
	return exit
}

func statusCommand(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, command string) int {
	opts, err := oneTask(args, command)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	reason := opts.Values["reason"]
	if reason == "" {
		reason = command
	}
	var result any
	switch command {
	case "complete":
		result, err = complete.Run(ctx, store, sessions, clock.System{}, opts.Positionals[0])
	case "fail":
		result, err = fail.Run(ctx, store, sessions, clock.System{}, opts.Positionals[0], reason)
	case "cancel":
		result, err = cancel.Run(ctx, store, sessions, clock.System{}, opts.Positionals[0], reason)
	}
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, command, result, fmt.Sprintf("%s: %s\n", command, opts.Positionals[0]), opts.JSON)
}

func codeName(exit int) string {
	switch exit {
	case ExitInvalid:
		return "invalid_input"
	case ExitValidation:
		return "validation_failed"
	case ExitReview:
		return "review_failed"
	case ExitCancelled:
		return "cancelled"
	case ExitWorkspace:
		return "workspace_error"
	default:
		return "runtime_error"
	}
}

func coalesce(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
