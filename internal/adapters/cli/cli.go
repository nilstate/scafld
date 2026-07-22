package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	adaptercli "github.com/nilstate/scafld/v2/internal/adapters/cli/adapter"
	configcli "github.com/nilstate/scafld/v2/internal/adapters/cli/config"
	"github.com/nilstate/scafld/v2/internal/adapters/cli/fallback"
	finalizecli "github.com/nilstate/scafld/v2/internal/adapters/cli/finalize"
	finalizestdiocli "github.com/nilstate/scafld/v2/internal/adapters/cli/finalizestdio"
	hardencli "github.com/nilstate/scafld/v2/internal/adapters/cli/harden"
	hardensubmitcli "github.com/nilstate/scafld/v2/internal/adapters/cli/hardensubmit"
	clihelp "github.com/nilstate/scafld/v2/internal/adapters/cli/help"
	initcmd "github.com/nilstate/scafld/v2/internal/adapters/cli/initcmd"
	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	reviewcli "github.com/nilstate/scafld/v2/internal/adapters/cli/review"
	reviewsubmitcli "github.com/nilstate/scafld/v2/internal/adapters/cli/reviewsubmit"
	verifycli "github.com/nilstate/scafld/v2/internal/adapters/cli/verify"
	"github.com/nilstate/scafld/v2/internal/adapters/clock"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/app/approve"
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
	{"init", "Bootstrap a scafld workspace"}, {"config", "Propose evidence-backed workspace configuration"},
	{"plan", "Create a draft task spec"}, {"harden", "Stress-test a draft spec before approval"},
	{"validate", "Validate a task spec"}, {"approve", "Approve a draft spec"},
	{"build", "Open or advance governed build phases"}, {"review", "Run the adversarial review gate"},
	{"finalize", "Finalize work with acceptance, review, and a signed receipt"},
	{"complete", "Complete reviewed work"}, {"fail", "Mark work failed"}, {"cancel", "Cancel work"},
	{"status", "Show spec status"}, {"list", "List specs"}, {"report", "Aggregate spec and run metrics"},
	{"handoff", "Render model-facing handoff material"}, {"adapter", "Render provider trigger packet"},
	{"verify", "Verify a signed scafld receipt"}, {"update", "Refresh managed scafld core files"},
}

type command struct{ name, summary string }
type commandHandler func(context.Context, []string, io.Writer, io.Writer) int

var commandHandlers = map[string]commandHandler{
	"init":                runInit,
	"config":              runConfig,
	"plan":                runPlan,
	"harden":              runHarden,
	"validate":            runValidate,
	"approve":             runApprove,
	"build":               runBuild,
	"review":              runReview,
	"finalize":            finalizecli.Handler(os.Stdin),
	"complete":            statusHandler("complete"),
	"fail":                statusHandler("fail"),
	"cancel":              statusHandler("cancel"),
	"status":              runStatus,
	"list":                runList,
	"report":              runReport,
	"handoff":             runHandoff,
	"adapter":             adaptercli.Handler(ExitInvalid, ExitWorkspace, ExitGeneric),
	"verify":              verifycli.Handler(),
	"update":              runUpdate,
	"finalize-stdio":      finalizestdiocli.Handler(os.Args[0], os.Stdin),
	"review-submit-stdio": reviewsubmitcli.Handler(os.Stdin),
	"harden-submit-stdio": hardensubmitcli.Handler(os.Stdin),
}

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	if ctx == nil {
		ctx = context.Background()
	}
	args = normalizeGlobalFlags(args)
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		clihelp.Print(stdout, helpCommands())
		return ExitSuccess
	}
	if args[0] == "--version" || args[0] == "version" {
		fmt.Fprintln(stdout, displayVersion())
		return ExitSuccess
	}
	if len(args) > 1 && (args[1] == "-h" || args[1] == "--help") && knownCommand(args[0]) {
		clihelp.PrintCommand(stdout, args[0], helpCommands())
		return ExitSuccess
	}
	if handler := commandHandlers[args[0]]; handler != nil {
		return handler(ctx, args[1:], stdout, stderr)
	}
	if args[0] == "exec" {
		fmt.Fprintln(stderr, `error: unknown command "exec"; acceptance commands are recorded by "scafld build <task_id>" after approval. To inspect reviewer context use "scafld review <task_id> --print-context".`)
		return ExitInvalid
	}
	fmt.Fprintf(stderr, "error: unknown command %q\n", args[0])
	return ExitInvalid
}

func knownCommand(name string) bool { return commandHandlers[name] != nil }

func helpCommands() []clihelp.Command {
	out := make([]clihelp.Command, 0, len(commands))
	for _, cmd := range commands {
		out = append(out, clihelp.Command{Name: cmd.name, Summary: cmd.summary})
	}
	return out
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
	result, err := initcmd.Run(ctx, root, !opts.Flags["no-agent-docs"], opts.Flags["ci"])
	if err != nil {
		return failOut(stderr, err, ExitWorkspace, opts.JSON)
	}
	return okOut(stdout, "init", result, initcmd.Message(result, opts.Flags["ci"]), opts.JSON)
}

func runConfig(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	root, err := commandRoot(ctx, opts, false)
	if err != nil {
		return failOut(stderr, err, ExitWorkspace, opts.JSON)
	}
	out, err := configcli.Run(ctx, root)
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	text := out.Prompt + fmt.Sprintf("\n---\nproposal: %s\nfollow agent_instructions before updating config or rule surfaces\n", out.Path)
	return okOut(stdout, "config", out, text, opts.JSON)
}

func runPlan(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := parseOptions(args)
	if err != nil || len(opts.Positionals) != 1 {
		return failOut(stderr, fallback.Error(err, errors.New("plan requires task_id")), ExitInvalid, opts.JSON)
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
		return failOut(stderr, fallback.Error(err, errors.New("validate requires task_id")), ExitInvalid, opts.JSON)
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
	input, err := hardencli.BuildInput(ctx, hardencli.RunOptions{
		Root:            root,
		TaskID:          opts.Positionals[0],
		MarkPassed:      opts.Flags["mark-passed"],
		Provider:        opts.Values["provider"],
		Command:         opts.Values["provider-command"],
		ProviderBinary:  opts.Values["provider-binary"],
		Model:           opts.Values["model"],
		SuppressContext: opts.Flags["no-context"],
		Progress:        stderr,
	})
	if err != nil {
		return failOut(stderr, err, ExitReview, opts.JSON)
	}
	out, err := harden.Run(ctx, store, clock.System{}, input)
	if err != nil {
		return failOut(stderr, err, ExitValidation, opts.JSON)
	}
	if opts.JSON {
		return okOut(stdout, "harden", out, "", true)
	}
	text, envelope := hardencli.ResultText(stderr, out)
	if envelope {
		return okOut(stdout, "harden", out, text, false)
	}
	fmt.Fprint(stdout, text)
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
	root, _ := commandRoot(ctx, opts, false)
	out, err := approve.Run(ctx, store, sessions, git.Adapter{Root: root}, clock.System{}, opts.Positionals[0], opts.Values["reason"])
	if err != nil {
		exit := ExitGeneric
		if errors.Is(err, approve.ErrApprovalReasonRequired) {
			exit = ExitValidation
		}
		return failOut(stderr, err, exit, opts.JSON)
	}
	return okOut(stdout, "approve", out, fmt.Sprintf("approved spec: %s\n", out.TaskID), opts.JSON)
}

func runBuild(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "build")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	root, _ := commandRoot(ctx, opts, false)
	cfg, err := configadapter.Load(ctx, root)
	if err != nil {
		return failOut(stderr, output.ConfigGateError(fmt.Errorf("load config: %w", err)), ExitGeneric, opts.JSON)
	}
	runner := process.Runner{DiagnosticsDir: root + "/.scafld/runs/" + opts.Positionals[0] + "/diagnostics"}
	executionConfig := configadapter.EffectiveExecution(root, cfg.Execution)
	out, err := build.Run(ctx, store, sessions, git.Adapter{Root: root}, runner, clock.System{}, build.Input{TaskID: opts.Positionals[0], CWD: root, Env: executionConfig.ProcessEnv(), Timeout: executionConfig.AbsoluteTimeout(), IdleTimeout: executionConfig.IdleTimeout()})
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
	return okOut(stdout, "build", out, output.Build(out), asJSON, code)
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
	var selected reviewcli.Selection
	if !opts.Flags["human-reviewed"] {
		selected, err = reviewcli.Select(ctx, reviewcli.Options{
			Root:           root,
			TaskID:         opts.Positionals[0],
			Provider:       opts.Values["provider"],
			Command:        opts.Values["provider-command"],
			ProviderBinary: opts.Values["provider-binary"],
			Model:          opts.Values["model"],
			Progress:       stderr,
			PrintContext:   opts.Flags["print-context"],
		})
		if err != nil {
			return failOut(stderr, err, ExitInvalid, opts.JSON)
		}
	}
	input := reviewcli.BuildInput(reviewcli.InputOptions{
		TaskID: opts.Positionals[0], Mode: opts.Values["mode"], ReviewScope: opts.Values["review-scope"],
		MaxFindings: opts.Values["max-findings"], MinAttackAngles: opts.Values["min-attack-angles"], ReviewDepth: opts.Values["review-depth"],
		ForceReview: opts.Flags["force"], PrintContext: opts.Flags["print-context"], HumanReviewed: opts.Flags["human-reviewed"], Reason: opts.Values["reason"],
	}, selected)
	out, err := review.RunWithInput(ctx, store, sessions, git.Adapter{Root: root}, selected.Provider, clock.System{}, input)
	if err != nil {
		return failOut(stderr, err, ExitReview, opts.JSON)
	}
	if out.Context != "" {
		if opts.JSON {
			return okOut(stdout, "review_context", out, out.Context, true)
		}
		fmt.Fprint(stdout, out.Context)
		return ExitSuccess
	}
	exit := ExitSuccess
	if out.Verdict != "pass" {
		exit = ExitReview
	}
	return okOut(stdout, "review", out, output.Review(out), opts.JSON, exit)
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
	out, err := status.RunWithOptions(ctx, store, sessions, opts.Positionals[0], status.Options{SuppressContext: opts.Flags["no-context"]}, git.Adapter{Root: store.Root})
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, "status", out, output.Status(out), opts.JSON)
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
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := report.Run(ctx, store, sessions)
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	return okOut(stdout, "report", out, output.Report(out), opts.JSON)
}

func runHandoff(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts, err := oneTask(args, "handoff")
	if err != nil {
		return failOut(stderr, err, ExitInvalid, opts.JSON)
	}
	store, sessions, code, err := stores(ctx, opts)
	if err != nil {
		return failOut(stderr, err, code, opts.JSON)
	}
	out, err := handoff.RunWithOptions(ctx, store, sessions, opts.Positionals[0], handoff.Options{SuppressContext: opts.Flags["no-context"]}, git.Adapter{Root: store.Root})
	if err != nil {
		return failOut(stderr, err, ExitGeneric, opts.JSON)
	}
	fmt.Fprint(stdout, out)
	return ExitSuccess
}
