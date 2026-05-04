package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/adapters/clock"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	"github.com/nilstate/scafld/v2/internal/app/approve"
	"github.com/nilstate/scafld/v2/internal/app/bootstrap"
	"github.com/nilstate/scafld/v2/internal/app/build"
	execusecase "github.com/nilstate/scafld/v2/internal/app/execspec"
	"github.com/nilstate/scafld/v2/internal/app/handoff"
	listusecase "github.com/nilstate/scafld/v2/internal/app/list"
	"github.com/nilstate/scafld/v2/internal/app/plan"
	"github.com/nilstate/scafld/v2/internal/app/report"
	"github.com/nilstate/scafld/v2/internal/app/review"
	"github.com/nilstate/scafld/v2/internal/app/status"
	"github.com/nilstate/scafld/v2/internal/app/validate"
)

var version = "0.0.0-dev"

const (
	ExitSuccess    = 0
	ExitGeneric    = 1
	ExitInvalid    = 2
	ExitValidation = 3
	ExitReview     = 4
	ExitCancelled  = 5
	ExitWorkspace  = 6
)

var commands = []command{
	{"init", "Bootstrap a scafld workspace"},
	{"plan", "Create a draft task spec"},
	{"validate", "Validate a task spec"},
	{"approve", "Approve a draft spec"},
	{"build", "Execute approved work"},
	{"exec", "Run selected acceptance criteria"},
	{"review", "Run the adversarial review gate"},
	{"complete", "Complete reviewed work"},
	{"fail", "Mark work failed"},
	{"cancel", "Cancel work"},
	{"status", "Show spec status"},
	{"list", "List specs"},
	{"report", "Aggregate spec and run metrics"},
	{"handoff", "Render model-facing handoff material"},
	{"update", "Refresh managed scafld core files"},
}

type command struct{ name, summary string }

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	args = normalizeGlobalFlags(args)
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		printHelp(stdout)
		return ExitSuccess
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(stdout, version)
		return ExitSuccess
	}
	if len(args) > 1 && (args[1] == "-h" || args[1] == "--help") && knownCommand(args[0]) {
		printCommandHelp(stdout, args[0])
		return ExitSuccess
	}
	switch args[0] {
	case "init":
		return runInit(ctx, args[1:], stdout, stderr)
	case "plan":
		return runPlan(ctx, args[1:], stdout, stderr)
	case "validate":
		return runValidate(ctx, args[1:], stdout, stderr)
	case "approve":
		return runApprove(ctx, args[1:], stdout, stderr)
	case "build":
		return runBuild(ctx, args[1:], stdout, stderr, false)
	case "exec":
		return runBuild(ctx, args[1:], stdout, stderr, true)
	case "review":
		return runReview(ctx, args[1:], stdout, stderr)
	case "complete":
		return runComplete(ctx, args[1:], stdout, stderr)
	case "fail":
		return runFail(ctx, args[1:], stdout, stderr)
	case "cancel":
		return runCancel(ctx, args[1:], stdout, stderr)
	case "status":
		return runStatus(ctx, args[1:], stdout, stderr)
	case "list":
		return runList(ctx, args[1:], stdout, stderr)
	case "report":
		return runReport(ctx, args[1:], stdout, stderr)
	case "handoff":
		return runHandoff(ctx, args[1:], stdout, stderr)
	case "update":
		fmt.Fprintln(stdout, "scafld core is up to date")
		return ExitSuccess
	default:
		fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
		return ExitInvalid
	}
}

func runInit(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	root := opts.Root
	if root == "" {
		root = "."
	}
	result, err := bootstrap.Run(ctx, filesystem.WorkspaceStore{}, bootstrap.Input{Root: root})
	if err != nil {
		return failOut(stderr, fmt.Errorf("init workspace: %w", err), ExitWorkspace, opts.JSON)
	}
	return okOut(stdout, "init", result, fmt.Sprintf("initialized scafld workspace: %s\n", result.Root), opts.JSON)
}

func runPlan(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil || len(opts.Positionals) != 1 {
		return failOut(stderr, coalesce(err, errors.New("plan requires task_id")), ExitInvalid, opts.JSON)
	}
	root, err := commandRoot(ctx, opts, true)
	if err != nil {
		return failOut(stderr, err, ExitWorkspace, opts.JSON)
	}
	out, err := plan.Run(ctx, markdown.Store{Root: root}, clock.System{}, plan.Input{
		TaskID: opts.Positionals[0], Title: opts.Values["title"], Summary: opts.Values["summary"],
		Command: opts.Values["command"], Size: opts.Values["size"], Risk: opts.Values["risk"],
	})
	if err != nil {
		return failOut(stderr, err, ExitValidation, opts.JSON)
	}
	return okOut(stdout, "plan", out, fmt.Sprintf("created draft spec: %s\n", out.Path), opts.JSON)
}

func runValidate(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil || len(opts.Positionals) != 1 {
		return failOut(stderr, coalesce(err, errors.New("validate requires task_id")), ExitInvalid, opts.JSON)
	}
	store, _, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := validate.Run(ctx, store, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitValidation, opts.JSON)
	}
	if !out.Valid {
		return failOut(stderr, errors.New(strings.Join(out.Errors, "; ")), ExitValidation, opts.JSON)
	}
	return okOut(stdout, "validate", out, fmt.Sprintf("valid spec: %s\n", out.TaskID), opts.JSON)
}

func runApprove(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "approve")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := approve.Run(ctx, store, sessions, clock.System{}, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, "approve", out, fmt.Sprintf("approved spec: %s\n", out.TaskID), opts.JSON)
}

func runBuild(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, selected bool) int {
	opts, err := oneTask(args, "build")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	root, _ := commandRoot(ctx, opts, false)
	runner := process.Runner{DiagnosticsDir: root + "/.scafld/runs/" + opts.Positionals[0] + "/diagnostics"}
	if selected {
		out, err := execusecase.Run(ctx, store, sessions, runner, clock.System{}, execusecase.Input{TaskID: opts.Positionals[0], CWD: root})
		return buildOut(stdout, stderr, out, err, opts.JSON)
	}
	out, err := build.Run(ctx, store, sessions, runner, clock.System{}, build.Input{TaskID: opts.Positionals[0], CWD: root})
	return buildOut(stdout, stderr, out, err, opts.JSON)
}

func buildOut(stdout io.Writer, stderr io.Writer, out build.Output, err error, asJSON bool) int {
	if err != nil {
		return failOut(stderr, err, ExitGeneric, asJSON)
	}
	code := ExitSuccess
	if out.Failed > 0 {
		code = ExitValidation
	}
	return okOut(stdout, "build", out, fmt.Sprintf("build %s: %d passed, %d failed\n", out.Status, out.Passed, out.Failed), asJSON, code)
}

func runReview(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "review")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	root, _ := commandRoot(ctx, opts, false)
	provider, err := selectedReviewProvider(opts, root, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	out, err := review.Run(ctx, store, sessions, git.Adapter{Root: root}, provider, clock.System{}, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitReview, opts.JSON)
	}
	exit := ExitSuccess
	if out.Verdict != "pass" {
		exit = ExitReview
	}
	return okOut(stdout, "review", out, fmt.Sprintf("review verdict: %s\n", out.Verdict), opts.JSON, exit)
}

func selectedReviewProvider(opts options, root string, taskID string) (review.Provider, error) {
	runner := process.Runner{DiagnosticsDir: root + "/.scafld/runs/" + taskID + "/diagnostics"}
	if command := opts.Values["provider-command"]; command != "" {
		return providers.CommandProvider{
			Command:     command,
			CWD:         root,
			Runner:      runner,
			Timeout:     30 * time.Minute,
			IdleTimeout: 2 * time.Minute,
		}, nil
	}
	switch provider := opts.Values["provider"]; provider {
	case "", "local":
		return providers.LocalProvider{}, nil
	case "command":
		return nil, errors.New("--provider=command requires --provider-command")
	case "claude":
		return providers.ClaudeProvider{
			Binary:      opts.Values["provider-binary"],
			Model:       opts.Values["model"],
			CWD:         root,
			Runner:      runner,
			Timeout:     30 * time.Minute,
			IdleTimeout: 2 * time.Minute,
		}, nil
	case "codex", "auto":
		return providers.CodexProvider{
			Binary:      opts.Values["provider-binary"],
			Model:       opts.Values["model"],
			CWD:         root,
			Runner:      runner,
			Timeout:     30 * time.Minute,
			IdleTimeout: 2 * time.Minute,
		}, nil
	default:
		return nil, fmt.Errorf("unknown review provider %q", provider)
	}
}

func runComplete(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return statusCommand(ctx, args, stdout, stderr, "complete")
}

func runFail(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return statusCommand(ctx, args, stdout, stderr, "fail")
}

func runCancel(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return statusCommand(ctx, args, stdout, stderr, "cancel")
}

func runStatus(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "status")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := status.Run(ctx, store, sessions, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, "status", out, fmt.Sprintf("%s: %s\nnext: %s\n", out.TaskID, out.Status, out.Next), opts.JSON)
}

func runList(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, _, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	records, err := listusecase.Run(ctx, store)
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	if opts.JSON {
		return okOut(stdout, "list", records, "", true)
	}
	for _, record := range records {
		fmt.Fprintf(stdout, "%s\t%s\t%s\n", record.TaskID, record.Status, record.Title)
	}
	return ExitSuccess
}

func runReport(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, _, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := report.Run(ctx, store)
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, "report", out, fmt.Sprintf("total specs: %d\n", out.Total), opts.JSON)
}

func runHandoff(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "handoff")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, _, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := handoff.Run(ctx, store, opts.Positionals[0])
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	fmt.Fprint(stdout, out)
	return ExitSuccess
}
