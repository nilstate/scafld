// Package cli translates command-line arguments into application use cases.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	hardencli "github.com/nilstate/scafld/v2/internal/adapters/cli/harden"
	reviewcli "github.com/nilstate/scafld/v2/internal/adapters/cli/review"
	"github.com/nilstate/scafld/v2/internal/adapters/clock"
	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	"github.com/nilstate/scafld/v2/internal/adapters/filesystem"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/app/approve"
	"github.com/nilstate/scafld/v2/internal/app/bootstrap"
	"github.com/nilstate/scafld/v2/internal/app/build"
	"github.com/nilstate/scafld/v2/internal/app/handoff"
	"github.com/nilstate/scafld/v2/internal/app/harden"
	listusecase "github.com/nilstate/scafld/v2/internal/app/list"
	"github.com/nilstate/scafld/v2/internal/app/plan"
	"github.com/nilstate/scafld/v2/internal/app/report"
	"github.com/nilstate/scafld/v2/internal/app/review"
	"github.com/nilstate/scafld/v2/internal/app/status"
	"github.com/nilstate/scafld/v2/internal/app/validate"
)

var version string

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
	{"harden", "Stress-test a draft spec before approval"},
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
type commandHandler func(context.Context, []string, io.Writer, io.Writer) int

var commandHandlers = map[string]commandHandler{
	"init":     runInit,
	"plan":     runPlan,
	"harden":   runHarden,
	"validate": runValidate,
	"approve":  runApprove,
	"build": func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		return runBuild(ctx, args, stdout, stderr, false)
	},
	"exec": func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		return runBuild(ctx, args, stdout, stderr, true)
	},
	"review":   runReview,
	"complete": runComplete,
	"fail":     runFail,
	"cancel":   runCancel,
	"status":   runStatus,
	"list":     runList,
	"report":   runReport,
	"handoff":  runHandoff,
	"update":   runUpdate,
}

// Run executes the CLI command and returns the process exit code.
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
		fmt.Fprintln(stdout, displayVersion())
		return ExitSuccess
	}
	if len(args) > 1 && (args[1] == "-h" || args[1] == "--help") && knownCommand(args[0]) {
		printCommandHelp(stdout, args[0])
		return ExitSuccess
	}
	if handler := commandHandlers[args[0]]; handler != nil {
		return handler(ctx, args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
	return ExitInvalid
}

func displayVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}

func runUpdate(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	root, err := commandRoot(ctx, opts, false)
	if err != nil {
		return failOut(stderr, err, ExitWorkspace, opts.JSON)
	}
	result, err := corebundle.Update(ctx, root)
	if err != nil {
		return failOut(stderr, fmt.Errorf("update core bundle: %w", err), ExitGeneric, opts.JSON)
	}
	agentDocs, err := corebundle.RefreshAgentDocs(ctx, root)
	if err != nil {
		return failOut(stderr, fmt.Errorf("refresh agent docs: %w", err), ExitGeneric, opts.JSON)
	}
	result.Created = append(result.Created, agentDocs.Created...)
	result.Updated = append(result.Updated, agentDocs.Updated...)
	result.Skipped = append(result.Skipped, agentDocs.Skipped...)
	text := fmt.Sprintf("refreshed scafld core: %d updated, %d created\n", len(result.Updated), len(result.Created))
	return okOut(stdout, "update", result, text, opts.JSON)
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
	bundle, err := corebundle.Init(ctx, result.Root)
	if err != nil {
		return failOut(stderr, fmt.Errorf("install core bundle: %w", err), ExitWorkspace, opts.JSON)
	}
	result.Merge(bundle.Created, bundle.Updated, bundle.Skipped)
	if !opts.Flags["no-agent-docs"] {
		agentDocs, err := corebundle.InitAgentDocs(ctx, result.Root)
		if err != nil {
			return failOut(stderr, fmt.Errorf("install agent docs: %w", err), ExitWorkspace, opts.JSON)
		}
		result.Merge(agentDocs.Created, agentDocs.Updated, agentDocs.Skipped)
	}
	gitignore, err := corebundle.InitGitignore(ctx, result.Root)
	if err != nil {
		return failOut(stderr, fmt.Errorf("install gitignore: %w", err), ExitWorkspace, opts.JSON)
	}
	result.Merge(gitignore.Created, gitignore.Updated, gitignore.Skipped)
	return okOut(stdout, "init", result, bootstrap.Message(result), opts.JSON)
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

func runHarden(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "harden")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, _, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	root, _ := commandRoot(ctx, opts, false)
	out, err := harden.Run(ctx, store, clock.System{}, harden.Input{
		TaskID:     opts.Positionals[0],
		MarkPassed: opts.Flags["mark-passed"],
		Root:       root,
		Prompt:     hardencli.Prompt(ctx, root),
	})
	if err != nil {
		return failOut(stderr, err, ExitValidation, opts.JSON)
	}
	if opts.JSON {
		return okOut(stdout, "harden", out, "", true)
	}
	if out.MarkedPassed {
		for _, warning := range out.Warnings {
			fmt.Fprintf(stderr, "warn: %s\n", warning)
		}
		return okOut(stdout, "harden", out, fmt.Sprintf("harden passed: %s\nnext: %s\n", out.TaskID, out.NextCommand), false)
	}
	fmt.Fprint(stdout, out.Prompt)
	fmt.Fprintf(stdout, "\n---\nspec: %s\nround: %s\nwhen done, mark the round passed: %s\n", out.Path, out.RoundID, out.NextCommand)
	return ExitSuccess
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

func runBuild(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer, _ bool) int {
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
	selected, err := reviewcli.Select(ctx, reviewcli.Options{
		Root:           root,
		TaskID:         opts.Positionals[0],
		Provider:       opts.Values["provider"],
		Command:        opts.Values["provider-command"],
		ProviderBinary: opts.Values["provider-binary"],
		Model:          opts.Values["model"],
	})
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	out, err := review.RunWithInput(ctx, store, sessions, git.Adapter{Root: root}, selected.Provider, clock.System{}, review.Input{
		TaskID: opts.Positionals[0],
		Passes: selected.Passes,
	})
	if err != nil {
		return failOut(stderr, err, ExitReview, opts.JSON)
	}
	exit := ExitSuccess
	if out.Verdict != "pass" {
		exit = ExitReview
	}
	return okOut(stdout, "review", out, fmt.Sprintf("review verdict: %s\n", out.Verdict), opts.JSON, exit)
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
