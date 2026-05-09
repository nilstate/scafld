package review

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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
	TaskID    string                  `json:"task_id"`
	Verdict   string                  `json:"verdict"`
	Mode      review.Mode             `json:"mode,omitempty"`
	Summary   string                  `json:"summary,omitempty"`
	Provider  string                  `json:"provider,omitempty"`
	Model     string                  `json:"model,omitempty"`
	Findings  []review.Finding        `json:"findings"`
	AttackLog []review.AttackLogEntry `json:"attack_log,omitempty"`
	Budget    review.Budget           `json:"budget,omitempty"`
	Next      string                  `json:"next"`
	Context   string                  `json:"context,omitempty"`
	Repair    *gate.Failure           `json:"repair,omitempty"`
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
	if len(scopeDrift) > 0 {
		if _, err := sessions.Append(ctx, model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "blocked",
			Reason: fmt.Sprintf("review local gates blocked before provider; %d scope drift change(s)", len(scopeDrift)),
			Output: reviewAttemptOutput(baselineScoped, taskChanges, scopeDrift),
		}, now); err != nil {
			return Output{}, err
		}
		dossier := review.Dossier{
			Verdict:   review.VerdictFail,
			Mode:      mode,
			Summary:   "Review blocked before provider because workspace drift is outside the declared task scope.",
			Provider:  "scafld",
			Findings:  []review.Finding{scopeDriftFinding(scopeDrift)},
			AttackLog: []review.AttackLogEntry{{Target: "workspace scope", Attack: "compare approval baseline to current state", Result: "finding", Notes: fmt.Sprintf("%d scope drift change(s)", len(scopeDrift))}},
			Budget:    reviewBudget(input, 1, 1),
		}
		return recordReviewDossier(ctx, specs, sessions, model, path, dossier, now)
	}
	if _, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:   "review_attempt",
		Status: "running",
		Reason: fmt.Sprintf("review provider running; baseline %d changed path(s), %d task change(s), %d scope drift change(s)", len(coreworkspace.Paths(baselineScoped)), len(taskChanges), len(scopeDrift)),
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
		}, now)
		return Output{}, mutationErr
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
		}, now)
		return Output{}, err
	}
	if dossier.Mode != mode {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: fmt.Sprintf("review dossier invalid: mode %q does not match requested mode %q", dossier.Mode, mode),
		}, now)
		return Output{}, fmt.Errorf("%w: mode %q does not match requested mode %q", review.ErrInvalidDossier, dossier.Mode, mode)
	}
	if err := review.ValidateDossier(dossier); err != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review dossier invalid: " + err.Error(),
		}, now)
		return Output{}, err
	}
	return recordReviewDossier(ctx, specs, sessions, model, path, dossier, now)
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
	model.CurrentState.ReviewGate = dossier.Verdict
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: dossier.Verdict, Mode: dossier.Mode, Summary: dossier.Summary, Provider: dossier.Provider, Model: dossier.Model, Findings: dossier.Findings, AttackLog: dossier.AttackLog, Budget: dossier.Budget, Next: command, Repair: reviewRepair(model, dossier, command, latestReviewEvidence(ledger))}, nil
}

func scopeDriftFinding(scopeDrift []coreworkspace.Mutation) review.Finding {
	path := "."
	if len(scopeDrift) > 0 {
		path = scopeDrift[0].Path
	}
	return review.Finding{
		ID:               "scope_drift",
		Severity:         review.SeverityHigh,
		BlocksCompletion: true,
		Category:         "scope",
		Confidence:       review.ConfidenceHigh,
		Location:         &review.Location{Path: path},
		Evidence:         "workspace changed outside declared task scope since approval: " + strings.Join(coreworkspace.MutationStrings(scopeDrift), ", "),
		Impact:           "The review would be grading unrelated work as if it belonged to this task.",
		Validation:       "Commit, revert, or declare the unrelated drift, then rerun scafld review.",
		Summary:          "Workspace changed outside declared task scope.",
	}
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

func reviewMode(ctx context.Context, sessions SessionStore, taskID string, input Input) review.Mode {
	if input.ForceMode {
		if input.Mode == review.ModeVerify {
			return review.ModeVerify
		}
		return review.ModeDiscover
	}
	switch strings.TrimSpace(input.RerunPolicy) {
	case "", "verify_open_blockers":
		if latestReviewHasOpenBlockers(ctx, sessions, taskID) {
			return review.ModeVerify
		}
	case "discover":
		return review.ModeDiscover
	case "verify":
		return review.ModeVerify
	}
	if input.Mode == review.ModeVerify {
		return review.ModeVerify
	}
	return review.ModeDiscover
}

func latestReviewHasOpenBlockers(ctx context.Context, sessions SessionStore, taskID string) bool {
	if sessions == nil {
		return false
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return false
	}
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		dossier, ok := review.DecodeDossier(entry.Output)
		return ok && review.OpenBlockerCount(dossier.Findings) > 0
	}
	return false
}

func reviewBudget(input Input, findings int, attacks int) review.Budget {
	return review.Budget{
		MaxFindings:        input.MaxFindings,
		MinAttackAngles:    input.MinAttackAngles,
		ActualFindings:     findings,
		ActualAttackAngles: attacks,
		Depth:              strings.TrimSpace(input.ReviewDepth),
	}
}

func applyRequestedBudget(dossier review.Dossier, requested review.Budget) review.Dossier {
	if dossier.Budget.MaxFindings == 0 {
		dossier.Budget.MaxFindings = requested.MaxFindings
	}
	if dossier.Budget.MinAttackAngles == 0 {
		dossier.Budget.MinAttackAngles = requested.MinAttackAngles
	}
	if strings.TrimSpace(dossier.Budget.Depth) == "" {
		dossier.Budget.Depth = requested.Depth
	}
	dossier.Budget.ActualFindings = len(dossier.Findings)
	dossier.Budget.ActualAttackAngles = len(dossier.AttackLog)
	return dossier
}

func workspaceMutationFinding(mutated []string) review.Finding {
	path := "."
	if len(mutated) > 0 {
		path = coreworkspace.ParseChange(mutated[0]).Path
		if path == "" {
			path = strings.TrimSpace(mutated[0])
		}
	}
	return review.Finding{
		ID:               "workspace_mutation",
		Severity:         review.SeverityCritical,
		BlocksCompletion: true,
		Category:         "review_integrity",
		Confidence:       review.ConfidenceHigh,
		Location:         &review.Location{Path: path},
		Evidence:         "workspace changed during review: " + strings.Join(mutated, ", "),
		Impact:           "The review provider changed the workspace while acting as a read-only reviewer, so its verdict is not trustworthy.",
		Validation:       "Restore the workspace to the expected state, ensure the provider is read-only, then rerun scafld review.",
		Summary:          "Workspace changed during review.",
	}
}

func appendSummary(current string, extra string) string {
	current = strings.TrimSpace(current)
	extra = strings.TrimSpace(extra)
	if current == "" {
		return extra
	}
	if extra == "" {
		return current
	}
	return current + " " + extra
}

func workspaceSnapshot(ctx context.Context, workspace WorkspaceStatus) ([]string, error) {
	if workspace == nil {
		return nil, nil
	}
	files, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), files...), nil
}

func taskBaseline(ctx context.Context, sessions SessionStore, taskID string, fallback []string) []string {
	if sessions == nil {
		return append([]string(nil), fallback...)
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return append([]string(nil), fallback...)
	}
	entry, ok := session.FirstWorkspaceBaseline(ledger)
	if !ok {
		return append([]string(nil), fallback...)
	}
	return session.WorkspaceBaselineSnapshot(entry)
}

func reviewComparisonSnapshot(snapshot []string) []string {
	var kept []string
	for _, raw := range snapshot {
		if reviewComparisonPath(coreworkspace.ParseChange(raw).Path) {
			kept = append(kept, raw)
		}
	}
	return kept
}

func reviewComparisonPath(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, prefix := range []string{
		".scafld/runs/",
		".scafld/specs/",
	} {
		if strings.HasPrefix(normalized+"/", prefix) {
			return false
		}
	}
	return true
}

func reviewBlockingMutations(before []string, after []string, scope []string, specPath string) []coreworkspace.Mutation {
	currentSpec := currentSpecReviewPath(specPath)
	normalizedScope := coreworkspace.NormalizeScope(scope)
	var blocking []coreworkspace.Mutation
	for _, mutation := range coreworkspace.Diff(before, after) {
		path := strings.Trim(strings.ReplaceAll(mutation.Path, "\\", "/"), "/")
		if currentSpec != "" && path == currentSpec {
			blocking = append(blocking, mutation)
			continue
		}
		if !reviewComparisonPath(path) {
			continue
		}
		if len(normalizedScope) == 0 || coreworkspace.PathInScope(path, normalizedScope) {
			blocking = append(blocking, mutation)
		}
	}
	return blocking
}

func currentSpecReviewPath(path string) string {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if idx := strings.Index(normalized, ".scafld/specs/"); idx >= 0 {
		return normalized[idx:]
	}
	return ""
}

func reviewAttemptOutput(baseline []string, taskChanges []coreworkspace.Mutation, scopeDrift []coreworkspace.Mutation) string {
	var b strings.Builder
	b.WriteString("baseline:\n")
	for _, path := range coreworkspace.Paths(baseline) {
		fmt.Fprintf(&b, "- %s\n", path)
	}
	b.WriteString("task_changes_since_baseline:\n")
	for _, line := range coreworkspace.MutationStrings(taskChanges) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	b.WriteString("scope_drift_since_baseline:\n")
	for _, line := range coreworkspace.MutationStrings(scopeDrift) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func deriveReviewScope(model spec.Model, explicit []string, snapshot []string) []string {
	if normalized := coreworkspace.NormalizeScope(explicit); len(normalized) > 0 {
		return normalized
	}
	var scope []string
	for _, pkg := range model.Context.Packages {
		if looksLikePath(pkg) || packageMatchesWorkspace(pkg, snapshot) {
			scope = append(scope, pkg)
		}
	}
	scope = append(scope, pathishItems(model.Context.FilesImpacted)...)
	scope = append(scope, pathishItems(model.Context.RelatedDocs)...)
	scope = append(scope, pathishItems(model.Scope)...)
	scope = append(scope, pathishItems(model.Touchpoints)...)
	for _, phase := range model.Phases {
		scope = append(scope, pathishItems(phase.Changes)...)
	}
	return filterReviewScope(coreworkspace.NormalizeScope(scope))
}

func filterReviewScope(scope []string) []string {
	filtered := make([]string, 0, len(scope))
	for _, item := range scope {
		if reviewScopePathAllowed(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func reviewScopePathAllowed(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if normalized == "" {
		return false
	}
	for _, segment := range strings.Split(normalized, "/") {
		if strings.HasPrefix(segment, ".env") {
			return false
		}
	}
	for _, denied := range []string{
		".git",
		".priv",
		".scafld/config.local.yaml",
		".scafld/reviews",
	} {
		if normalized == denied || strings.HasPrefix(normalized+"/", denied+"/") {
			return false
		}
	}
	return true
}

func packageMatchesWorkspace(pkg string, snapshot []string) bool {
	prefixes := coreworkspace.NormalizeScope([]string{pkg})
	if len(prefixes) == 0 {
		return false
	}
	for _, raw := range snapshot {
		if coreworkspace.PathInScope(coreworkspace.ParseChange(raw).Path, prefixes) {
			return true
		}
	}
	return false
}

func pathishItems(values []string) []string {
	var paths []string
	for _, value := range values {
		for _, token := range pathishTokens(value) {
			paths = append(paths, token)
		}
	}
	return paths
}

func pathishTokens(value string) []string {
	var tokens []string
	text := strings.TrimSpace(value)
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		rest := text[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			break
		}
		token := strings.TrimSpace(rest[:end])
		if looksLikePath(token) {
			tokens = append(tokens, token)
		}
		text = rest[end+1:]
	}
	if len(tokens) > 0 {
		return tokens
	}
	first := strings.Fields(strings.TrimLeft(value, "-* "))
	if len(first) == 0 {
		return nil
	}
	token := strings.Trim(first[0], "`:,;")
	if looksLikePath(token) {
		return []string{token}
	}
	return nil
}

func looksLikePath(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" || strings.Contains(text, "://") {
		return false
	}
	return strings.Contains(text, "/") || strings.HasPrefix(text, ".") || strings.Contains(text, ".")
}

func nextForVerdict(taskID string, verdict string) (string, string) {
	if verdict == "pass" {
		return "complete", "scafld complete " + taskID
	}
	return "repair", "scafld handoff " + taskID
}

func reviewRepair(model spec.Model, dossier review.Dossier, command string, evidence string) *gate.Failure {
	if dossier.Verdict != review.VerdictFail {
		return nil
	}
	blockers := make([]string, 0, len(dossier.Findings))
	for _, finding := range dossier.Findings {
		if !review.BlocksCompletion(finding) {
			continue
		}
		label := finding.ID
		if finding.Summary != "" {
			label += ": " + finding.Summary
		}
		if finding.Validation != "" {
			label += " | validate: " + finding.Validation
		}
		blockers = append(blockers, label)
	}
	return &gate.Failure{
		Gate:     "review",
		Status:   string(model.Status),
		Reason:   "review verdict fail",
		Evidence: []string{evidence},
		Expected: "review verdict pass",
		Actual:   "review verdict fail",
		Blockers: blockers,
		Next:     command,
	}
}

func latestReviewEvidence(ledger session.Session) string {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		if entry.ID != "" {
			return entry.ID
		}
		return "review event"
	}
	return "review event"
}

func reviewContextPacket(model spec.Model, specPath string, passes []Pass, invariants map[string]string, reviewScope []string, baseline []string, taskChanges []coreworkspace.Mutation, scopeDrift []coreworkspace.Mutation, extra []reviewcontext.Section, mode review.Mode, maxFindings int, minAttackAngles int, depth string, rerunPolicy string) reviewcontext.Packet {
	sourcePath := currentSpecReviewPath(specPath)
	if sourcePath == "" {
		sourcePath = strings.TrimSpace(specPath)
	}
	if sourcePath == "" {
		sourcePath = model.TaskID
	}
	sections := []reviewcontext.Section{
		contextSection("task_contract", "Task Contract", 10, taskContractBody(model), "spec", sourcePath),
		contextSection("review_request", "Review Request", 12, reviewRequestBody(mode, maxFindings, minAttackAngles, depth, rerunPolicy), "scafld", "review"),
		contextSection("configured_invariants", "Configured Invariants", 15, configuredInvariantsBody(invariants), "config", ".scafld/config.yaml"),
		contextSection("review_focus", "Review Focus", 18, reviewFocusBody(passes), "config", ".scafld/config.yaml"),
		contextSection("task_scope", "Task Scope", 20, taskScopeBody(model, reviewScope), "spec", sourcePath),
		contextSection("workspace_baseline", "Workspace Baseline Before Review", 30, workspaceBaselineBody(baseline), "session", model.TaskID),
		contextSection("task_changes", "Task Changes Since Approval Baseline", 40, workspaceChangesBody("Task Changes Since Approval Baseline", taskChanges), "session", model.TaskID),
		contextSection("scope_drift", "Scope Drift Since Approval Baseline", 50, workspaceChangesBody("Scope Drift Since Approval Baseline", scopeDrift), "session", model.TaskID),
		contextSection("acceptance_evidence", "Acceptance Criteria", 60, acceptanceBody(model), "session", model.TaskID),
		contextSection("provider_instruction", "Provider Instruction", 90, providerInstructionBody(), "scafld", "review"),
	}
	sections = append(sections, extra...)
	return reviewcontext.Packet{TaskID: model.TaskID, Title: model.Title, Status: string(model.Status), Sections: sections}
}

func contextSection(key string, title string, order int, body string, kind string, path string) reviewcontext.Section {
	body = strings.TrimSpace(body)
	sourcePath := strings.TrimSpace(path)
	if sourcePath == "" {
		sourcePath = key
	}
	if !strings.Contains(sourcePath, "#") {
		sourcePath += "#" + key
	}
	return reviewcontext.Section{
		Key:     key,
		Title:   title,
		Order:   order,
		Body:    body,
		Sources: []reviewcontext.Source{reviewcontext.SourceForContent("derived_"+kind, sourcePath, []byte(body))},
	}
}

func taskContractBody(model spec.Model) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\nStatus: %s\n", model.Title, model.Status)
	if strings.TrimSpace(model.Summary) != "" {
		fmt.Fprintf(&b, "\nSummary:\n%s\n", strings.TrimSpace(model.Summary))
	}
	if len(model.Objectives) > 0 {
		b.WriteString("\nObjectives:\n")
		for _, objective := range model.Objectives {
			fmt.Fprintf(&b, "- %s\n", objective)
		}
	}
	if len(model.Context.Invariants) > 0 {
		b.WriteString("\nDeclared invariants:\n")
		for _, invariant := range model.Context.Invariants {
			if strings.TrimSpace(invariant) != "" {
				fmt.Fprintf(&b, "- %s\n", invariant)
			}
		}
	}
	return b.String()
}

func taskScopeBody(model spec.Model, reviewScope []string) string {
	var b strings.Builder
	writeTaskScope(&b, model, reviewScope)
	return stripSectionHeading(b.String(), "Task Scope")
}

func workspaceBaselineBody(baseline []string) string {
	var b strings.Builder
	writeWorkspaceBaseline(&b, baseline)
	return stripSectionHeading(b.String(), "Workspace Baseline Before Review")
}

func workspaceChangesBody(title string, mutations []coreworkspace.Mutation) string {
	var b strings.Builder
	writeWorkspaceChanges(&b, title, mutations)
	return stripSectionHeading(b.String(), title)
}

func acceptanceBody(model spec.Model) string {
	var b strings.Builder
	for _, criterion := range model.AllCriteria() {
		fmt.Fprintf(&b, "- %s (%s): %s\n", criterion.ID, criterion.ExpectedKind, criterion.Command)
		if strings.TrimSpace(criterion.Status) != "" {
			fmt.Fprintf(&b, "  - Status: %s\n", criterion.Status)
		}
		if strings.TrimSpace(criterion.Evidence) != "" {
			fmt.Fprintf(&b, "  - Evidence: %s\n", criterion.Evidence)
		}
	}
	return b.String()
}

func reviewFocusBody(passes []Pass) string {
	var b strings.Builder
	writeReviewPasses(&b, passes)
	return stripSectionHeading(b.String(), "Review Focus")
}

func configuredInvariantsBody(invariants map[string]string) string {
	if len(invariants) == 0 {
		return ""
	}
	keys := make([]string, 0, len(invariants))
	for key := range invariants {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		description := strings.TrimSpace(invariants[key])
		if description == "" {
			fmt.Fprintf(&b, "- `%s`\n", key)
			continue
		}
		fmt.Fprintf(&b, "- `%s`: %s\n", key, description)
	}
	return b.String()
}

func reviewRequestBody(mode review.Mode, maxFindings int, minAttackAngles int, depth string, rerunPolicy string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Mode: %s\n", mode)
	if maxFindings > 0 {
		fmt.Fprintf(&b, "Max findings: %d\n", maxFindings)
	}
	if minAttackAngles > 0 {
		fmt.Fprintf(&b, "Minimum attack angles: %d\n", minAttackAngles)
	}
	if strings.TrimSpace(depth) != "" {
		fmt.Fprintf(&b, "Review depth: %s\n", strings.TrimSpace(depth))
	}
	if strings.TrimSpace(rerunPolicy) != "" {
		fmt.Fprintf(&b, "Rerun policy: %s\n", strings.TrimSpace(rerunPolicy))
	}
	return b.String()
}

func providerInstructionBody() string {
	return "Review mode is read-only. Do not run build, test, or mutation commands; treat recorded acceptance evidence above as already executed. Treat review as task-scoped: unchanged dirty paths from the approval baseline are context, not findings by themselves. Scope drift since the approval baseline is blocking unless the spec explicitly declares it. Do not emit placeholder output while investigating; the final output must be one complete ReviewDossier JSON object. Separate severity from the gate: use severity `critical`, `high`, `medium`, or `low`, then set `blocks_completion` true only when completion must stop. Completion-blocking findings must include location, evidence, impact, and validation. Record attack_log entries for the bounded checks you actually performed, using result `finding`, `clean`, or `skipped`."
}

func stripSectionHeading(text string, title string) string {
	prefix := "## " + title + "\n\n"
	text = strings.TrimSpace(text)
	return strings.TrimSpace(strings.TrimPrefix(text, prefix))
}

func writeTaskScope(b *strings.Builder, model spec.Model, reviewScope []string) {
	if len(reviewScope) == 0 &&
		len(model.Context.Packages) == 0 &&
		len(model.Context.FilesImpacted) == 0 &&
		len(model.Scope) == 0 &&
		len(model.Touchpoints) == 0 &&
		!phasesDeclareChanges(model.Phases) {
		return
	}
	b.WriteString("## Task Scope\n\n")
	if len(reviewScope) > 0 {
		b.WriteString("Explicit review scope:\n")
		for _, item := range reviewScope {
			fmt.Fprintf(b, "- `%s`\n", item)
		}
		b.WriteString("\n")
	}
	writeStringList(b, "Packages", model.Context.Packages, true)
	writeStringList(b, "Files impacted", model.Context.FilesImpacted, true)
	writeStringList(b, "Scope", model.Scope, false)
	writeStringList(b, "Touchpoints", model.Touchpoints, false)
	for _, phase := range model.Phases {
		if len(phase.Changes) == 0 {
			continue
		}
		title := strings.TrimSpace(phase.Name)
		if title == "" {
			title = phase.ID
		}
		fmt.Fprintf(b, "%s changes:\n", title)
		for _, change := range phase.Changes {
			if strings.TrimSpace(change) != "" {
				fmt.Fprintf(b, "- %s\n", change)
			}
		}
		b.WriteString("\n")
	}
}

func writeWorkspaceBaseline(b *strings.Builder, baseline []string) {
	b.WriteString("## Workspace Baseline Before Review\n\n")
	paths := coreworkspace.Paths(baseline)
	if len(paths) == 0 {
		b.WriteString("- clean\n\n")
		return
	}
	for i, path := range paths {
		if i >= 80 {
			fmt.Fprintf(b, "- ... %d more path(s)\n", len(paths)-i)
			break
		}
		fmt.Fprintf(b, "- `%s`\n", path)
	}
	b.WriteString("\n")
}

func writeWorkspaceChanges(b *strings.Builder, title string, mutations []coreworkspace.Mutation) {
	b.WriteString("## " + title + "\n\n")
	if len(mutations) == 0 {
		b.WriteString("- none\n\n")
		return
	}
	for _, line := range coreworkspace.MutationStrings(mutations) {
		fmt.Fprintf(b, "- %s\n", line)
	}
	b.WriteString("\n")
}

func writeStringList(b *strings.Builder, title string, values []string, code bool) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", title)
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" {
			continue
		}
		if code {
			fmt.Fprintf(b, "- `%s`\n", text)
		} else {
			fmt.Fprintf(b, "- %s\n", text)
		}
	}
	b.WriteString("\n")
}

func phasesDeclareChanges(phases []spec.Phase) bool {
	for _, phase := range phases {
		if len(phase.Changes) > 0 {
			return true
		}
	}
	return false
}

func writeReviewPasses(b *strings.Builder, passes []Pass) {
	if len(passes) == 0 {
		return
	}
	sorted := append([]Pass(nil), passes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Order == sorted[j].Order {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Order < sorted[j].Order
	})
	b.WriteString("\n## Review Focus\n\n")
	for _, pass := range sorted {
		title := strings.TrimSpace(pass.Title)
		if title == "" {
			title = pass.ID
		}
		category := strings.TrimSpace(pass.Category)
		if category == "" {
			category = "review"
		}
		fmt.Fprintf(b, "- %s: %s", category, title)
		if description := strings.TrimSpace(pass.Description); description != "" {
			fmt.Fprintf(b, " - %s", description)
		}
		b.WriteString("\n")
	}
}
