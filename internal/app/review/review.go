package review

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/app/specsource"
	"github.com/nilstate/scafld/v2/internal/core/agentcontract"
	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	"github.com/nilstate/scafld/v2/internal/core/diagnostics"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/lifecycle"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

// ErrSpecNotReviewable is returned when review is attempted before build reaches the review gate.
var ErrSpecNotReviewable = errors.New("review requires task status review")

// ErrReviewStartBlocked is returned when the current review gate state forbids
// starting a new provider attempt.
var ErrReviewStartBlocked = errors.New("review attempt cannot start")

// SpecStore is the spec persistence port used by review.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by review.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	AppendTransaction(context.Context, string, string, func(session.Session) ([]session.Entry, error)) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Provider is the review provider port.
type Provider interface {
	Invoke(context.Context, review.Request) (review.Dossier, error)
}

// WorkspaceStatus is the mutation-guard workspace state port.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
	ResolveHead(context.Context) (string, bool, error)
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
	TaskID                  string
	Mode                    review.Mode
	ForceMode               bool
	Passes                  []Pass
	Invariants              map[string]string
	ReviewScope             []string
	ContextSections         []reviewcontext.Section
	ContextMaxBytes         int
	RequiredContextMaxBytes int
	Contract                agentcontract.Contract
	MaxFindings             int
	MinAttackAngles         int
	ReviewDepth             string
	RerunPolicy             string
	ForceReview             bool
	ProviderName            string
	ProviderModel           string
	PrintContext            bool
	HumanReviewed           bool
	Reason                  string
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
	source, err := specsource.Load(ctx, specs, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	model := source.Model
	path := source.Path
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return Output{}, fmt.Errorf("load session evidence: %w", err)
	}
	model = reconcile.FromSession(model, ledger)
	source.Model = model
	if model.Status != spec.StatusReview {
		if model.Status == spec.StatusCompleted {
			return Output{}, fmt.Errorf("%w: task is archived/completed; create a new task to continue%s", ErrSpecNotReviewable, reviewCompletionSuffix(ctx, sessions, model.TaskID))
		}
		return Output{}, reviewRequiresBuildError(model)
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
	scopeProjection := reviewScopeProjection(model, input.ReviewScope, baselineComparison, beforeComparison)
	scope := scopeProjection.Scope
	baselineScoped := scopeProjection.Baseline
	taskChanges := scopeProjection.TaskChanges
	scopeDrift := scopeProjection.AmbientDrift
	mode := reviewMode(ctx, sessions, model.TaskID, input)
	knownFindings := knownFindingsForMode(ctx, sessions, model.TaskID, mode)
	contextPacket := reviewContextPacket(source, input.Contract, input.Passes, input.Invariants, scope, baselineScoped, taskChanges, scopeDrift, knownFindings, input.ContextSections, mode, input.MaxFindings, input.MinAttackAngles, input.ReviewDepth, input.RerunPolicy)
	if input.PrintContext {
		prompt := reviewcontext.RenderMarkdown(contextPacket, reviewcontext.Options{MaxBytes: input.ContextMaxBytes, RequiredMaxBytes: input.RequiredContextMaxBytes})
		return Output{TaskID: model.TaskID, Verdict: "not_run", Next: "scafld review " + model.TaskID, Context: prompt}, nil
	}
	prompt, err := reviewcontext.RenderMarkdownStrict(contextPacket, reviewcontext.Options{MaxBytes: input.ContextMaxBytes, RequiredMaxBytes: input.RequiredContextMaxBytes})
	if err != nil {
		return Output{}, reviewContextGateError(model, err)
	}
	seal, err := reviewSeal(ctx, workspace, beforeComparison)
	if err != nil {
		return Output{}, reviewGateError(model, err, "review workspace head unavailable", err.Error())
	}
	preReviewMaterialScope := reviewMaterialScope(scope, beforeComparison)
	if rerunScope := reviewgate.ReviewRerunMaterialScope(ledger); len(rerunScope) > 0 {
		preReviewMaterialScope = rerunScope
	}
	preReviewMaterial, hasPreReviewMaterial, preReviewMaterialErr := reviewMaterialSeal(ctx, workspace, preReviewMaterialScope)
	if preReviewMaterialErr != nil {
		return Output{}, reviewGateError(model, preReviewMaterialErr, "review material seal failed", preReviewMaterialErr.Error())
	}
	nowTime := clock.Now().UTC()
	now := nowTime.Format(time.RFC3339)
	attempt, err := startReviewAttempt(ctx, sessions, model, input, mode, baselineScoped, taskChanges, scopeDrift, seal, preReviewMaterial, hasPreReviewMaterial, nowTime)
	if err != nil {
		return Output{}, err
	}
	dossier, err := invokeReviewProvider(ctx, provider, providerReviewInput{
		taskID: model.TaskID,
		prompt: prompt,
		packet: contextPacket,
	})
	postReviewCtx, cancelPostReview := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelPostReview()
	afterFull, mutationErr := workspaceSnapshot(postReviewCtx, workspace)
	if mutationErr != nil {
		reason, diagnosticPath := diagnostics.FailureReason("review workspace snapshot failed", mutationErr, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, mutationErr, "review workspace snapshot failed", mutationErr.Error())
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
		reason, diagnosticPath := diagnostics.FailureReason("review provider failed", err, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, err, "review provider failed", err.Error())
	}
	if dossier.Mode != mode {
		err := fmt.Errorf("%w: mode %q does not match requested mode %q", review.ErrInvalidDossier, dossier.Mode, mode)
		reason, diagnosticPath := diagnostics.FailureReason("review dossier invalid", err, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, err, "review dossier invalid", err.Error())
	}
	if err := review.ValidateDossier(dossier); err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("review dossier invalid", err, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, err, "review dossier invalid", err.Error())
	}
	if err := validateRequestedBudget(dossier); err != nil {
		reason, diagnosticPath := diagnostics.FailureReason("review dossier budget invalid", err, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, err, "review dossier budget invalid", err.Error())
	}
	material, hasMaterial, materialErr := reviewMaterialSeal(postReviewCtx, workspace, reviewMaterialScope(scope, beforeComparison))
	if materialErr != nil {
		reason, diagnosticPath := diagnostics.FailureReason("review material seal failed", materialErr, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, materialErr, "review material seal failed", materialErr.Error())
	}
	out, reviewRecorded, recordErr := recordReviewDossier(ctx, specs, sessions, model, path, dossier, now, seal, material, hasMaterial, attempt)
	if recordErr != nil {
		if reviewRecorded {
			return Output{}, recordErr
		}
		reason, diagnosticPath := diagnostics.FailureReason("review dossier recording failed", recordErr, 240)
		return recordFailedReviewAttempt(ctx, sessions, model, attempt, reason, diagnosticPath, now, recordErr, "review dossier recording failed", recordErr.Error())
	}
	return out, nil
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
	if path := diagnostics.Path(err); path != "" {
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

func reviewContextGateError(model spec.Model, err error) error {
	return gate.New(err, gate.Failure{
		Gate:     "review",
		Status:   string(model.Status),
		Reason:   "review context packet invalid",
		Evidence: []string{"review context packet"},
		Expected: "required source context within the configured provider packet budget",
		Actual:   err.Error(),
		Blockers: []string{"required source context exceeds provider packet budget"},
		Next:     "shrink the spec or raise the review context budget, then run scafld review " + model.TaskID,
	})
}

func reviewRequiresBuildError(model spec.Model) error {
	reason := strings.TrimSpace(model.CurrentState.Reason)
	if reason == "" {
		reason = "build evidence is required before review"
	}
	return gate.New(ErrSpecNotReviewable, gate.Failure{
		Gate:     "build",
		Status:   string(model.Status),
		Reason:   reason,
		Expected: "task status review with current acceptance evidence",
		Actual:   "reconciled status " + string(model.Status),
		Blockers: []string{reason},
		Next:     "scafld build " + model.TaskID,
	})
}

func startReviewAttempt(ctx context.Context, sessions SessionStore, model spec.Model, input Input, mode review.Mode, baselineScoped []string, taskChanges []coreworkspace.Mutation, scopeDrift []coreworkspace.Mutation, seal reviewSessionSeal, material reviewevidence.MaterialSeal, hasMaterial bool, now time.Time) (reviewgate.Attempt, error) {
	reason := fmt.Sprintf("review provider running; baseline %d changed path(s), %d task change(s), %d ambient drift change(s)", len(coreworkspace.Paths(baselineScoped)), len(taskChanges), len(scopeDrift))
	if input.ForceReview && strings.TrimSpace(input.Reason) != "" {
		reason += "; forced review: " + strings.TrimSpace(input.Reason)
	}
	attemptEntry := reviewgate.RunningAttemptEntry(reviewgate.AttemptEntryInput{
		AttemptID:      reviewgate.NewAttemptID(model.TaskID, now),
		LeaseExpiresAt: now.Add(reviewgate.DefaultAttemptLease),
		Mode:           mode,
		Provider:       input.ProviderName,
		Model:          input.ProviderModel,
		PassCount:      1,
		Reason:         reason,
		Output:         reviewAttemptOutput(baselineScoped, taskChanges, scopeDrift),
	})
	opts := reviewStartProjectionOptions(now, seal, material, hasMaterial)
	ledger, err := sessions.AppendTransaction(ctx, model.TaskID, now.Format(time.RFC3339), func(current session.Session) ([]session.Entry, error) {
		state := reviewgate.Project(current, model, opts)
		switch state.Kind {
		case reviewgate.KindAttemptStale:
			return []session.Entry{
				reviewgate.AbandonedAttemptEntry(state.LatestAttempt, "stale running review attempt abandoned before starting a new review"),
				attemptEntry,
			}, nil
		case reviewgate.KindAttemptRunning, reviewgate.KindReviewFailed, reviewgate.KindReviewPassed, reviewgate.KindReviewNeedsOperatorDecision:
			if state.Kind == reviewgate.KindReviewPassed && input.ForceReview {
				return []session.Entry{attemptEntry}, nil
			}
			if state.Kind == reviewgate.KindReviewNeedsOperatorDecision && input.ForceReview && strings.TrimSpace(input.Reason) != "" {
				return []session.Entry{attemptEntry}, nil
			}
			return nil, reviewStartError(model, state)
		default:
			return []session.Entry{attemptEntry}, nil
		}
	})
	if err != nil {
		return reviewgate.Attempt{}, err
	}
	state := reviewgate.Project(ledger, model, opts)
	if !state.HasAttempt || state.LatestAttempt.Status != reviewgate.AttemptStatusRunning {
		return reviewgate.Attempt{}, errors.New("review attempt start did not record a running attempt")
	}
	return state.LatestAttempt, nil
}

func reviewStartProjectionOptions(now time.Time, seal reviewSessionSeal, material reviewevidence.MaterialSeal, hasMaterial bool) reviewgate.Options {
	opts := reviewgate.Options{
		Now: now,
		WorkspaceSeal: reviewgate.WorkspaceSeal{
			Head:  seal.reviewedHead,
			Dirty: seal.reviewedDirty,
			Diff:  seal.reviewedDiff,
		},
		HasWorkspaceSeal: true,
	}
	if hasMaterial {
		opts.MaterialSeal = material
		opts.HasMaterialSeal = true
	}
	return opts
}

func reviewStartError(model spec.Model, state reviewgate.State) error {
	reason := state.Reason
	if reason == "" {
		reason = string(state.Kind)
	}
	next := state.Next
	if next == "" {
		next = "scafld review " + model.TaskID
	}
	expected := "review gate ready for a new provider attempt"
	blockers := append([]string(nil), state.Blockers...)
	if len(blockers) == 0 {
		blockers = []string{reason}
	}
	switch state.Kind {
	case reviewgate.KindAttemptRunning:
		expected = "no unexpired running review attempt"
	case reviewgate.KindReviewFailed:
		expected = "fresh build evidence after repairing review findings"
		blockers = append(blockers,
			"run scafld handoff "+model.TaskID+" to read the findings",
			"after repair run scafld build "+model.TaskID,
			"then rerun scafld review "+model.TaskID,
		)
	case reviewgate.KindReviewNeedsOperatorDecision:
		expected = "material/spec repair evidence after failed review, or explicit --force --reason operator decision"
		blockers = append(blockers,
			"run scafld handoff "+model.TaskID+" to read current blockers",
			"repair material or spec, then run scafld build "+model.TaskID,
			"if rejecting the blocker as advisory/bookkeeping/overengineering, run scafld review "+model.TaskID+" --force --reason <reason>",
		)
	case reviewgate.KindReviewPassed:
		expected = "no current passing review or an explicit --force rerun"
	}
	return gate.New(ErrReviewStartBlocked, gate.Failure{
		Gate:     "review",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: nonEmptyStrings(state.Evidence, "session review entries"),
		Expected: expected,
		Actual:   fallbackString(state.Actual, reason),
		Blockers: blockers,
		Next:     next,
	})
}

func appendFailedReviewAttempt(ctx context.Context, sessions SessionStore, taskID string, attempt reviewgate.Attempt, reason string, diagnosticPath string, now string) error {
	writeCtx, cancel := lifecycle.TerminalEvidenceContext(ctx)
	defer cancel()
	_, err := sessions.Append(writeCtx, taskID, reviewgate.FailedAttemptEntry(attempt, reason, diagnosticPath), now)
	return err
}

func recordFailedReviewAttempt(ctx context.Context, sessions SessionStore, model spec.Model, attempt reviewgate.Attempt, reason string, diagnosticPath string, now string, cause error, gateReason string, actual string) (Output, error) {
	gateErr := reviewGateError(model, cause, gateReason, actual)
	if err := appendFailedReviewAttempt(ctx, sessions, model.TaskID, attempt, reason, diagnosticPath, now); err != nil {
		return Output{}, errors.Join(gateErr, fmt.Errorf("record review attempt failure: %w", err))
	}
	return Output{}, gateErr
}

func recordReviewDossier(ctx context.Context, specs SpecStore, sessions SessionStore, model spec.Model, path string, dossier review.Dossier, now string, seal reviewSessionSeal, material reviewevidence.MaterialSeal, hasMaterial bool, attempt reviewgate.Attempt) (Output, bool, error) {
	if err := review.ValidateDossier(dossier); err != nil {
		return Output{}, false, err
	}
	writeCtx, cancel := lifecycle.TerminalEvidenceContext(ctx)
	defer cancel()
	next, command := nextForVerdict(model.TaskID, dossier.Verdict)
	packet := review.EncodeDossier(dossier)
	entry := session.Entry{
		Type:                    "review",
		Status:                  dossier.Verdict,
		Reason:                  reviewReason(dossier),
		Provider:                dossier.Provider,
		Output:                  packet,
		ReviewPacket:            packet,
		CanonicalResponseSHA256: review.ResponseSHA256(packet),
		ProviderModel:           dossier.Model,
		ProviderSession:         dossier.SessionID,
		ReviewedHead:            seal.reviewedHead,
		ReviewedDirty:           seal.reviewedDirty,
		ReviewedDiff:            seal.reviewedDiff,
		ReviewedSpec:            spec.ContractDigest(model),
	}
	if hasMaterial {
		entry.ReviewedScope = append([]string(nil), material.Scope...)
		entry.ReviewedMaterialDigest = material.Digest
	}
	ledger, err := sessions.AppendTransaction(writeCtx, model.TaskID, now, func(session.Session) ([]session.Entry, error) {
		return []session.Entry{
			reviewgate.AcceptedAttemptEntry(attempt, "review dossier accepted for recording"),
			entry,
		}, nil
	})
	if err != nil {
		return Output{}, false, err
	}
	if loaded, loadErr := sessions.Load(writeCtx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.CurrentState.ReviewGate = dossier.Verdict
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	if err := specs.Save(writeCtx, path, model); err != nil {
		return Output{}, true, err
	}
	return Output{TaskID: model.TaskID, Verdict: dossier.Verdict, Mode: dossier.Mode, Summary: dossier.Summary, Provider: dossier.Provider, Model: dossier.Model, OutputFormat: dossier.OutputFormat, Normalizations: dossier.Normalizations, Findings: dossier.Findings, AttackLog: dossier.AttackLog, Budget: dossier.Budget, Next: command, Repair: reviewRepair(model, dossier, command, latestReviewEvidence(ledger))}, true, nil
}

func nonEmptyStrings(values []string, fallback string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 && strings.TrimSpace(fallback) != "" {
		return []string{fallback}
	}
	return out
}

func fallbackString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
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
