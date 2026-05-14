package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

// ErrSpecNotReviewable is returned when review is attempted before build reaches the review gate.
var ErrSpecNotReviewable = errors.New("review requires task status review")

// SpecStore is the spec persistence port used by review.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by review.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Provider is the review provider port.
type Provider interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
}

// WorkspaceStatus is the mutation-guard workspace state port.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies review timestamps.
type Clock interface{ Now() time.Time }

// Output describes a completed review run.
type Output struct {
	TaskID         string                  `json:"task_id"`
	Verdict        string                  `json:"verdict"`
	Mode           review.Mode             `json:"mode,omitempty"`
	Summary        string                  `json:"summary,omitempty"`
	Provider       string                  `json:"provider,omitempty"`
	Model          string                  `json:"model,omitempty"`
	OutputFormat   string                  `json:"output_format,omitempty"`
	Normalizations []string                `json:"normalizations,omitempty"`
	Findings       []review.Finding        `json:"findings"`
	AttackLog      []review.AttackLogEntry `json:"attack_log,omitempty"`
	Budget         review.Budget           `json:"budget,omitempty"`
	Next           string                  `json:"next"`
	Context        string                  `json:"context,omitempty"`
	Repair         *gate.Failure           `json:"repair,omitempty"`
}

// GateFailure exposes review blockers to the CLI JSON envelope.
func (o Output) GateFailure() *gate.Failure { return o.Repair }

// Input describes the task and review agenda to run.
type Input struct {
	TaskID          string
	Mode            review.Mode
	ForceMode       bool
	Passes          []Pass
	Invariants      map[string]string
	ReviewScope     []string
	ContextSections []reviewcontext.Section
	ContextMaxBytes int
	MaxFindings     int
	MinAttackAngles int
	ReviewDepth     string
	RerunPolicy     string
	PrintContext    bool
	HumanReviewed   bool
	Reason          string
}

// Pass describes one configured review pass included in the provider prompt.
type Pass struct {
	ID          string
	Category    string
	Order       int
	Title       string
	Description string
}

// Run executes the review gate for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, provider Provider, clock Clock, taskID string) (Output, error) {
	return RunWithInput(ctx, specs, sessions, workspace, provider, clock, Input{TaskID: taskID})
}

// RunWithInput executes the review gate using an explicit review agenda.
func RunWithInput(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, provider Provider, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	if model.Status != spec.StatusReview {
		if model.Status == spec.StatusCompleted {
			return Output{}, fmt.Errorf("%w: task is archived/completed; create a new task to continue%s", ErrSpecNotReviewable, reviewCompletionSuffix(ctx, sessions, model.TaskID))
		}
		return Output{}, fmt.Errorf("%w: run scafld build %s first", ErrSpecNotReviewable, model.TaskID)
	}
	if input.HumanReviewed {
		return runHumanReviewed(ctx, specs, sessions, clock, model, path, input.Reason)
	}
	beforeFull, err := workspaceSnapshot(ctx, workspace)
	if err != nil {
		return Output{}, err
	}
	baselineFull := taskBaseline(ctx, sessions, model.TaskID, beforeFull)
	baselineComparison := reviewComparisonSnapshot(baselineFull)
	beforeComparison := reviewComparisonSnapshot(beforeFull)
	scope := deriveReviewScope(model, input.ReviewScope, append(append([]string(nil), baselineComparison...), beforeComparison...))
	baselineScoped := coreworkspace.Filter(baselineComparison, scope)
	taskChanges, scopeDrift := coreworkspace.PartitionMutations(coreworkspace.Diff(baselineComparison, beforeComparison), scope)
	mode := reviewMode(ctx, sessions, model.TaskID, input)
	contextPacket := reviewContextPacket(model, path, input.Passes, input.Invariants, scope, baselineScoped, taskChanges, scopeDrift, input.ContextSections, mode, input.MaxFindings, input.MinAttackAngles, input.ReviewDepth, input.RerunPolicy)
	prompt := reviewcontext.RenderMarkdown(contextPacket, reviewcontext.Options{MaxBytes: input.ContextMaxBytes})
	if input.PrintContext {
		return Output{TaskID: model.TaskID, Verdict: "not_run", Next: "scafld review " + model.TaskID, Context: prompt}, nil
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if _, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:   "review_attempt",
		Status: "running",
		Reason: fmt.Sprintf("review provider running; baseline %d changed path(s), %d task change(s), %d ambient drift change(s)", len(coreworkspace.Paths(baselineScoped)), len(taskChanges), len(scopeDrift)),
		Output: reviewAttemptOutput(baselineScoped, taskChanges, scopeDrift),
	}, now); err != nil {
		return Output{}, err
	}
	dossier, err := provider.Invoke(ctx, review.Request{TaskID: model.TaskID, Prompt: prompt, Context: contextPacket})
	afterFull, mutationErr := workspaceSnapshot(ctx, workspace)
	if mutationErr != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review workspace snapshot failed: " + mutationErr.Error(),
			Path:   diagnosticPath(mutationErr),
		}, now)
		return Output{}, reviewGateError(model, mutationErr, "review workspace snapshot failed", mutationErr.Error())
	}
	if mutated := coreworkspace.MutationStrings(reviewBlockingMutations(beforeFull, afterFull, scope, path)); len(mutated) > 0 {
		dossier.Findings = append(dossier.Findings, workspaceMutationFinding(mutated))
		dossier.AttackLog = append(dossier.AttackLog, review.AttackLogEntry{Target: "workspace mutation guard", Attack: "compare pre-review and post-review workspace snapshots", Result: review.AttackResultFinding, Notes: strings.Join(mutated, ", ")})
		dossier.Summary = appendSummary(dossier.Summary, "Workspace changed during review; review failed closed.")
		dossier.Verdict = review.VerdictFromFindings(dossier.Findings)
		if dossier.Mode == "" {
			dossier.Mode = mode
		}
		if dossier.Provider == "" {
			dossier.Provider = "scafld"
		}
		err = nil
	}
	dossier = applyRequestedBudget(dossier, reviewBudget(input, 0, 0))
	if err != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review provider failed: " + err.Error(),
			Path:   diagnosticPath(err),
		}, now)
		return Output{}, reviewGateError(model, err, "review provider failed", err.Error())
	}
	if dossier.Mode != mode {
		err := fmt.Errorf("%w: mode %q does not match requested mode %q", review.ErrInvalidDossier, dossier.Mode, mode)
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: fmt.Sprintf("review dossier invalid: mode %q does not match requested mode %q", dossier.Mode, mode),
			Path:   diagnosticPath(err),
		}, now)
		return Output{}, reviewGateError(model, err, "review dossier invalid", err.Error())
	}
	if err := review.ValidateDossier(dossier); err != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review dossier invalid: " + err.Error(),
			Path:   diagnosticPath(err),
		}, now)
		return Output{}, reviewGateError(model, err, "review dossier invalid", err.Error())
	}
	return recordReviewDossier(ctx, specs, sessions, model, path, dossier, now)
}

func reviewCompletionSuffix(ctx context.Context, sessions SessionStore, taskID string) string {
	if sessions == nil {
		return ""
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return ""
	}
	return corecompletion.TerminalAuthority(ledger).RefusalSuffix()
}

func reviewGateError(model spec.Model, err error, reason string, actual string) error {
	if err == nil {
		err = errors.New(reason)
	}
	evidence := []string{"review_attempt session entry", "provider diagnostic output"}
	if path := diagnosticPath(err); path != "" {
		evidence = []string{path}
	}
	return gate.New(err, gate.Failure{
		Gate:     "review",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: evidence,
		Expected: "valid ReviewDossier submitted by an external reviewer",
		Actual:   actual,
		Blockers: []string{reason},
		Next:     "scafld handoff " + model.TaskID,
	})
}

func diagnosticPath(err error) string {
	var withDiagnostic interface{ DiagnosticPath() string }
	if errors.As(err, &withDiagnostic) {
		return withDiagnostic.DiagnosticPath()
	}
	return ""
}

func recordReviewDossier(ctx context.Context, specs SpecStore, sessions SessionStore, model spec.Model, path string, dossier review.Dossier, now string) (Output, error) {
	if err := review.ValidateDossier(dossier); err != nil {
		return Output{}, err
	}
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = dossier.Verdict
	model.Review.Mode = dossier.Mode
	model.Review.Summary = dossier.Summary
	model.Review.Findings = dossier.Findings
	model.Review.AttackLog = dossier.AttackLog
	model.Review.Budget = dossier.Budget
	model.Review.Provider = dossier.Provider
	model.Review.Model = dossier.Model
	model.Review.OutputFormat = dossier.OutputFormat
	model.Review.Normalizations = dossier.Normalizations
	model.CurrentState.ReviewGate = dossier.Verdict
	next, command := nextForVerdict(model.TaskID, dossier.Verdict)
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	ledger, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:     "review",
		Status:   dossier.Verdict,
		Reason:   reviewReason(dossier),
		Provider: dossier.Provider,
		Output:   review.EncodeDossier(dossier),
	}, now)
	if err != nil {
		return Output{}, err
	}
	if loaded, loadErr := sessions.Load(ctx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = dossier.Verdict
	model.Review.Mode = dossier.Mode
	model.Review.Summary = dossier.Summary
	model.Review.Findings = dossier.Findings
	model.Review.AttackLog = dossier.AttackLog
	model.Review.Budget = dossier.Budget
	model.Review.Provider = dossier.Provider
	model.Review.Model = dossier.Model
	model.Review.OutputFormat = dossier.OutputFormat
	model.Review.Normalizations = dossier.Normalizations
	model.CurrentState.ReviewGate = dossier.Verdict
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: dossier.Verdict, Mode: dossier.Mode, Summary: dossier.Summary, Provider: dossier.Provider, Model: dossier.Model, OutputFormat: dossier.OutputFormat, Normalizations: dossier.Normalizations, Findings: dossier.Findings, AttackLog: dossier.AttackLog, Budget: dossier.Budget, Next: command, Repair: reviewRepair(model, dossier, command, latestReviewEvidence(ledger))}, nil
}

func runHumanReviewed(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, model spec.Model, path string, reason string) (Output, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Output{}, errors.New("human-reviewed review requires --reason")
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	ledger, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:     "review_override",
		Status:   "accepted",
		Reason:   reason,
		Provider: "human",
	}, now)
	if err != nil {
		return Output{}, err
	}
	ledger, err = sessions.Append(ctx, model.TaskID, session.Entry{
		Type:     "review",
		Status:   review.VerdictPass,
		Reason:   "human-reviewed override: " + reason,
		Provider: "human",
	}, now)
	if err != nil {
		return Output{}, err
	}
	if loaded, loadErr := sessions.Load(ctx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = review.VerdictPass
	model.Review.Mode = review.ModeVerify
	model.Review.Summary = "Human-reviewed override accepted: " + reason
	model.Review.Findings = nil
	model.Review.AttackLog = []review.AttackLogEntry{{Target: "review gate", Attack: "manual human audit", Result: review.AttackResultClean, Notes: reason}}
	model.Review.Budget = review.Budget{ActualAttackAngles: 1, Depth: "human"}
	model.CurrentState.ReviewGate = review.VerdictPass
	model.CurrentState.Next = "complete"
	model.CurrentState.AllowedFollowUp = "scafld complete " + model.TaskID
	model.CurrentState.Reason = "human-reviewed override: " + reason
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: review.VerdictPass, Mode: review.ModeVerify, Summary: model.Review.Summary, Provider: "human", AttackLog: model.Review.AttackLog, Budget: model.Review.Budget, Next: model.CurrentState.AllowedFollowUp}, nil
}

func reviewReason(dossier review.Dossier) string {
	blocking := review.OpenBlockerCount(dossier.Findings)
	if len(dossier.Findings) == 0 {
		return "review gate " + dossier.Verdict
	}
	return fmt.Sprintf("review gate %s: %d finding(s), %d completion blocker(s)", dossier.Verdict, len(dossier.Findings), blocking)
}
