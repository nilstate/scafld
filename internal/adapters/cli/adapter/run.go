package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/app/envelope"
	"github.com/nilstate/scafld/v2/internal/app/handoff"
	appstatus "github.com/nilstate/scafld/v2/internal/app/status"
)

// Options describes the adapter packet requested by a provider wrapper.
type Options struct {
	Root     string
	Provider string
	Mode     string
	TaskID   string
}

// Output is the structured adapter packet result.
type Output struct {
	Provider string               `json:"provider"`
	Mode     string               `json:"mode"`
	TaskID   string               `json:"task_id"`
	Status   appstatus.Output     `json:"status"`
	Handoff  string               `json:"handoff"`
	Packet   string               `json:"packet"`
	Command  string               `json:"command,omitempty"`
	Action   appstatus.NextAction `json:"next_action"`
}

// Handler returns the CLI command handler for adapter packets.
func Handler(invalid int, workspace int, generic int) func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		opts, err := parseArgs(args)
		if err != nil || len(opts.positionals) != 3 {
			return fail(stderr, coalesce(err, errors.New("adapter requires provider, mode, and task_id")), invalid, opts.json)
		}
		root, err := filesystem.ResolveRoot(ctx, opts.root, filesystem.Discovery{})
		if err != nil {
			return fail(stderr, err, workspace, opts.json)
		}
		out, err := Run(ctx, Options{Root: root, Provider: opts.positionals[0], Mode: opts.positionals[1], TaskID: opts.positionals[2]})
		if err != nil {
			return fail(stderr, err, generic, opts.json)
		}
		return ok(stdout, out, opts.json)
	}
}

// Run renders the current status and handoff for provider trigger wrappers.
func Run(ctx context.Context, opts Options) (Output, error) {
	provider := strings.ToLower(strings.TrimSpace(opts.Provider))
	if provider == "" {
		return Output{}, errors.New("adapter requires provider")
	}
	if !adapterProvider(provider) {
		return Output{}, fmt.Errorf("adapter provider must be codex, claude, or gemini, got %q", opts.Provider)
	}
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode != "build" && mode != "review" {
		return Output{}, fmt.Errorf("adapter mode must be build or review, got %q", opts.Mode)
	}
	taskID := strings.TrimSpace(opts.TaskID)
	if taskID == "" {
		return Output{}, errors.New("adapter requires task_id")
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	specs := markdown.Store{Root: root}
	sessions := jsonstore.SessionStore{Root: root}
	statusOut, err := appstatus.Run(ctx, specs, sessions, taskID)
	if err != nil {
		return Output{}, err
	}
	handoffOut, err := handoff.Run(ctx, specs, sessions, taskID)
	if err != nil {
		return Output{}, err
	}
	out := Output{
		Provider: provider,
		Mode:     mode,
		TaskID:   taskID,
		Status:   statusOut,
		Handoff:  handoffOut,
		Command:  statusOut.NextAction.Command,
		Action:   statusOut.NextAction,
	}
	out.Packet = render(out)
	return out, nil
}

type cliOptions struct {
	root        string
	json        bool
	positionals []string
}

func parseArgs(args []string) (cliOptions, error) {
	var opts cliOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "--root" && i+1 < len(args):
			opts.root = args[i+1]
			i++
		case strings.HasPrefix(arg, "--root="):
			opts.root = strings.TrimPrefix(arg, "--root=")
		case strings.HasPrefix(arg, "-"):
			return opts, fmt.Errorf("unknown flag %q", arg)
		default:
			opts.positionals = append(opts.positionals, arg)
		}
	}
	return opts, nil
}

func ok(w io.Writer, out Output, asJSON bool) int {
	if asJSON {
		_ = json.NewEncoder(w).Encode(envelope.Envelope[Output]{OK: true, Command: "adapter", Result: out})
		return 0
	}
	fmt.Fprint(w, out.Packet)
	return 0
}

func fail(w io.Writer, err error, exit int, asJSON bool) int {
	if asJSON {
		_ = json.NewEncoder(w).Encode(envelope.Envelope[map[string]any]{OK: false, Error: &envelope.Error{Code: output.CodeName(exit), Message: err.Error(), ExitCode: exit}})
		return exit
	}
	fmt.Fprintf(w, "error: %v\n", err)
	return exit
}

func coalesce(err error, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

func render(out Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# scafld Adapter Packet\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", out.Provider)
	fmt.Fprintf(&b, "Mode: %s\n", out.Mode)
	fmt.Fprintf(&b, "Task: %s\n", out.TaskID)
	fmt.Fprintf(&b, "Status: %s\n", out.Status.Status)
	writeAction(&b, out.Action)
	writeContract(&b, out)
	writeModeGuidance(&b, out)
	fmt.Fprintf(&b, "\n## Handoff\n\n%s", strings.TrimSpace(out.Handoff))
	b.WriteString("\n")
	return b.String()
}

func writeAction(b *strings.Builder, action appstatus.NextAction) {
	if action.Action == "" && action.Role == "" && action.Command == "" {
		return
	}
	if action.Action != "" {
		fmt.Fprintf(b, "Next action: %s\n", action.Action)
	}
	if action.Role != "" {
		fmt.Fprintf(b, "Role: %s\n", action.Role)
	}
	if action.Command != "" {
		fmt.Fprintf(b, "Command: %s\n", action.Command)
	}
	if action.AfterCommand != "" {
		fmt.Fprintf(b, "After command: %s\n", action.AfterCommand)
	}
	if action.ThenCommand != "" {
		fmt.Fprintf(b, "Then command: %s\n", action.ThenCommand)
	}
	if action.Reason != "" {
		fmt.Fprintf(b, "Reason: %s\n", action.Reason)
	}
}

func writeContract(b *strings.Builder, out Output) {
	fmt.Fprintf(b, "\n## Contract\n\n")
	fmt.Fprintf(b, "- Treat `%s` as the source of truth before acting.\n", "scafld status --json "+out.TaskID)
	b.WriteString("- Treat this packet and the handoff as transport only; durable state lives in the spec and session ledger.\n")
	b.WriteString("- Follow the next-action command sequence exactly unless status changes.\n")
	b.WriteString("- Do not infer lifecycle progress from prose or from an older packet.\n")
}

func writeModeGuidance(b *strings.Builder, out Output) {
	fmt.Fprintf(b, "\n## Mode Guidance\n\n")
	if out.Mode == "review" {
		writeReviewGuidance(b, out)
		return
	}
	b.WriteString("- Build mode is for implementation or repair work against the handoff below.\n")
	if out.Action.AfterCommand != "" {
		fmt.Fprintf(b, "- After changes, run `%s` to refresh acceptance evidence.\n", out.Action.AfterCommand)
	}
	if out.Action.ThenCommand != "" {
		fmt.Fprintf(b, "- Then run `%s`.\n", out.Action.ThenCommand)
	}
}

func writeReviewGuidance(b *strings.Builder, out Output) {
	if out.Action.Action != "run_review" {
		fmt.Fprintf(b, "- Review mode was requested, but current next action is `%s`.\n", fallback(out.Action.Action, "unknown"))
		if out.Action.Command != "" {
			fmt.Fprintf(b, "- First run `%s`.\n", out.Action.Command)
		}
		if out.Action.AfterCommand != "" {
			fmt.Fprintf(b, "- Then run `%s`.\n", out.Action.AfterCommand)
		}
		if out.Action.ThenCommand != "" {
			fmt.Fprintf(b, "- Then run `%s`.\n", out.Action.ThenCommand)
		}
		return
	}
	fmt.Fprintf(b, "- Run the review through scafld so the accepted dossier is recorded in the session ledger.\n")
	fmt.Fprintf(b, "- Provider review command: `%s`.\n", "scafld review "+out.TaskID+" --provider "+out.Provider)
}

func fallback(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func adapterProvider(provider string) bool {
	switch provider {
	case "codex", "claude", "gemini":
		return true
	default:
		return false
	}
}
