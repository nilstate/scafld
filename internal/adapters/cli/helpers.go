package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	"github.com/nilstate/scafld/v2/internal/adapters/clock"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/app/cancel"
	"github.com/nilstate/scafld/v2/internal/app/complete"
	"github.com/nilstate/scafld/v2/internal/app/envelope"
	"github.com/nilstate/scafld/v2/internal/app/fail"
)

type options struct {
	Root        string
	JSON        bool
	Values      map[string]string
	Flags       map[string]bool
	Positionals []string
}

var valueFlags = map[string]bool{
	"root":             true,
	"title":            true,
	"summary":          true,
	"command":          true,
	"size":             true,
	"risk":             true,
	"reason":           true,
	"provider":         true,
	"provider-command": true,
	"provider-binary":  true,
	"model":            true,
	"review-scope":     true,
}

var boolFlags = map[string]bool{"human-reviewed": true, "mark-passed": true, "no-agent-docs": true, "print-context": true}

func parseOptions(args []string) (options, error) {
	opts := options{Values: map[string]string{}, Flags: map[string]bool{}}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--json" {
			opts.JSON = true
			continue
		}
		if handled, err := parseFlagValue(args, &i, &opts); err != nil {
			return opts, err
		} else if handled {
			continue
		}
		if strings.HasPrefix(arg, "-") {
			return opts, fmt.Errorf("unknown flag %q", arg)
		}
		opts.Positionals = append(opts.Positionals, arg)
	}
	return opts, nil
}

func parseFlagValue(args []string, index *int, opts *options) (bool, error) {
	arg := args[*index]
	if !strings.HasPrefix(arg, "--") {
		return false, nil
	}
	if key, value, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "="); ok {
		if boolFlags[key] {
			opts.Flags[key] = parseBoolFlag(value)
			return true, nil
		}
		setOptionValue(opts, key, value)
		return true, nil
	}
	key := strings.TrimPrefix(arg, "--")
	if boolFlags[key] {
		opts.Flags[key] = true
		return true, nil
	}
	if !valueFlags[key] {
		return false, nil
	}
	if *index+1 >= len(args) {
		return true, fmt.Errorf("%s requires a value", arg)
	}
	setOptionValue(opts, key, args[*index+1])
	*index = *index + 1
	return true, nil
}

func parseBoolFlag(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func setOptionValue(opts *options, key string, value string) {
	if key == "root" {
		opts.Root = value
		return
	}
	opts.Values[key] = value
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
		env := envelope.Envelope[T]{OK: exit == ExitSuccess, Command: command, Result: result}
		if exit != ExitSuccess {
			env.Error = &envelope.Error{Code: output.CodeName(exit), Message: "gate blocked", Gate: output.GateFailureFromResult(result), ExitCode: exit}
		}
		_ = json.NewEncoder(w).Encode(env)
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
		_ = json.NewEncoder(w).Encode(envelope.Envelope[map[string]any]{
			OK: false,
			Error: &envelope.Error{
				Code:     output.CodeName(exit),
				Message:  err.Error(),
				Gate:     output.GateFailure(err),
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

func coalesce(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}
