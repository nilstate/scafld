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
	Invoke(context.Context, review.Request) (review.Packet, error)
}

// WorkspaceStatus is the mutation-guard workspace state port.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies review timestamps.
type Clock interface{ Now() time.Time }

// Output describes a completed review run.
type Output struct {
	TaskID   string           `json:"task_id"`
	Verdict  string           `json:"verdict"`
	Findings []review.Finding `json:"findings"`
	Next     string           `json:"next"`
	Repair   *gate.Failure    `json:"repair,omitempty"`
}

// GateFailure exposes review blockers to the CLI JSON envelope.
func (o Output) GateFailure() *gate.Failure { return o.Repair }

// Input describes the task and review agenda to run.
type Input struct {
	TaskID        string
	Passes        []Pass
	ReviewScope   []string
	HumanReviewed bool
	Reason        string
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
		packet := review.Packet{
			Verdict:  review.VerdictFail,
			Provider: "scafld",
			Findings: []review.Finding{scopeDriftFinding(scopeDrift)},
		}
		return recordReviewPacket(ctx, specs, sessions, model, path, packet, now)
	}
	if _, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:   "review_attempt",
		Status: "running",
		Reason: fmt.Sprintf("review provider running; baseline %d changed path(s), %d task change(s), %d scope drift change(s)", len(coreworkspace.Paths(baselineScoped)), len(taskChanges), len(scopeDrift)),
		Output: reviewAttemptOutput(baselineScoped, taskChanges, scopeDrift),
	}, now); err != nil {
		return Output{}, err
	}
	packet, err := provider.Invoke(ctx, review.Request{TaskID: model.TaskID, Prompt: promptForModel(model, input.Passes, scope, baselineScoped, taskChanges, scopeDrift)})
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
		packet.Verdict = review.VerdictFail
		packet.Findings = append(packet.Findings, review.Finding{
			ID:       "workspace_mutation",
			Severity: review.SeverityBlocking,
			Summary:  "workspace changed during review: " + strings.Join(mutated, ", "),
		})
		err = nil
	}
	if err != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review provider failed: " + err.Error(),
		}, now)
		return Output{}, err
	}
	if err := review.ValidatePacket(packet); err != nil {
		_, _ = sessions.Append(context.WithoutCancel(ctx), model.TaskID, session.Entry{
			Type:   "review_attempt",
			Status: "failed",
			Reason: "review packet invalid: " + err.Error(),
		}, now)
		return Output{}, err
	}
	return recordReviewPacket(ctx, specs, sessions, model, path, packet, now)
}

func recordReviewPacket(ctx context.Context, specs SpecStore, sessions SessionStore, model spec.Model, path string, packet review.Packet, now string) (Output, error) {
	if err := review.ValidatePacket(packet); err != nil {
		return Output{}, err
	}
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = packet.Verdict
	model.Review.Findings = packet.Findings
	model.CurrentState.ReviewGate = packet.Verdict
	next, command := nextForVerdict(model.TaskID, packet.Verdict)
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	ledger, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:     "review",
		Status:   packet.Verdict,
		Reason:   reviewReason(packet),
		Provider: packet.Provider,
		Output:   review.EncodeFindings(packet.Findings),
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
	model.Review.Verdict = packet.Verdict
	model.Review.Findings = packet.Findings
	model.CurrentState.ReviewGate = packet.Verdict
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: packet.Verdict, Findings: packet.Findings, Next: command, Repair: reviewRepair(model, packet, command, latestReviewEvidence(ledger))}, nil
}

func scopeDriftFinding(scopeDrift []coreworkspace.Mutation) review.Finding {
	return review.Finding{
		ID:       "scope_drift",
		Severity: review.SeverityBlocking,
		Summary:  "workspace changed outside declared task scope since approval: " + strings.Join(coreworkspace.MutationStrings(scopeDrift), ", "),
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
	model.Review.Findings = nil
	model.CurrentState.ReviewGate = review.VerdictPass
	model.CurrentState.Next = "complete"
	model.CurrentState.AllowedFollowUp = "scafld complete " + model.TaskID
	model.CurrentState.Reason = "human-reviewed override: " + reason
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: review.VerdictPass, Next: model.CurrentState.AllowedFollowUp}, nil
}

func reviewReason(packet review.Packet) string {
	blocking := review.CountBlocking(packet.Findings)
	if len(packet.Findings) == 0 {
		return "review gate " + packet.Verdict
	}
	return fmt.Sprintf("review gate %s: %d finding(s), %d blocking", packet.Verdict, len(packet.Findings), blocking)
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
	return coreworkspace.NormalizeScope(scope)
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

func reviewRepair(model spec.Model, packet review.Packet, command string, evidence string) *gate.Failure {
	if packet.Verdict != review.VerdictFail {
		return nil
	}
	blockers := make([]string, 0, len(packet.Findings))
	for _, finding := range packet.Findings {
		if finding.Severity != review.SeverityBlocking {
			continue
		}
		label := finding.ID
		if finding.Summary != "" {
			label += ": " + finding.Summary
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

func promptForModel(model spec.Model, passes []Pass, reviewScope []string, baseline []string, taskChanges []coreworkspace.Mutation, scopeDrift []coreworkspace.Mutation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review %s\n\n", model.TaskID)
	fmt.Fprintf(&b, "Title: %s\nStatus: %s\n\n", model.Title, model.Status)
	if strings.TrimSpace(model.Summary) != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", strings.TrimSpace(model.Summary))
	}
	if len(model.Objectives) > 0 {
		b.WriteString("## Objectives\n\n")
		for _, objective := range model.Objectives {
			fmt.Fprintf(&b, "- %s\n", objective)
		}
		b.WriteString("\n")
	}
	if len(model.Context.Invariants) > 0 {
		b.WriteString("## Declared Invariants\n\n")
		for _, invariant := range model.Context.Invariants {
			if strings.TrimSpace(invariant) != "" {
				fmt.Fprintf(&b, "- %s\n", invariant)
			}
		}
		b.WriteString("\n")
	}
	writeTaskScope(&b, model, reviewScope)
	writeWorkspaceBaseline(&b, baseline)
	writeWorkspaceChanges(&b, "Task Changes Since Approval Baseline", taskChanges)
	writeWorkspaceChanges(&b, "Scope Drift Since Approval Baseline", scopeDrift)
	b.WriteString("## Acceptance Criteria\n\n")
	for _, criterion := range model.AllCriteria() {
		fmt.Fprintf(&b, "- %s (%s): %s\n", criterion.ID, criterion.ExpectedKind, criterion.Command)
		if strings.TrimSpace(criterion.Status) != "" {
			fmt.Fprintf(&b, "  - Status: %s\n", criterion.Status)
		}
		if strings.TrimSpace(criterion.Evidence) != "" {
			fmt.Fprintf(&b, "  - Evidence: %s\n", criterion.Evidence)
		}
	}
	writeReviewPasses(&b, passes)
	b.WriteString("\nReview mode is read-only. Do not run build, test, or mutation commands; treat recorded acceptance evidence above as already executed. Treat review as task-scoped: unchanged dirty paths from the approval baseline are context, not findings by themselves. Scope drift since the approval baseline is blocking unless the spec explicitly declares it. Do not emit placeholder ReviewPackets while investigating; the final output must be one complete ReviewPacket JSON object with `verdict` and `findings`. If your transport only supports line frames, emit `finding` frames with severity `blocking` or `non_blocking`, then a `verdict` frame with verdict `pass` or `fail`.\n")
	return b.String()
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
