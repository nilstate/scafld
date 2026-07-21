package status

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/app/specsource"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/reviewmaterial"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec loading port used by status.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore is the session loading port used by status.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// WorkspaceStatus captures the current Git-visible workspace state for read-only
// review-gate projection.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
	ResolveHead(context.Context) (string, bool, error)
}

type workspaceMaterialStatus interface {
	MaterialSeal(context.Context, []string) (reviewevidence.MaterialSeal, error)
}

// Output describes the task status projection.
type Output struct {
	TaskID          string          `json:"task_id"`
	Status          spec.Status     `json:"status"`
	Title           string          `json:"title"`
	Next            string          `json:"next"`
	NextAction      NextAction      `json:"next_action,omitempty"`
	Gate            string          `json:"gate,omitempty"`
	TrustedState    string          `json:"trusted_state,omitempty"`
	AllowedFollowUp string          `json:"allowed_follow_up,omitempty"`
	SessionOK       bool            `json:"session_ok"`
	SpecSource      *SpecSource     `json:"spec_source,omitempty"`
	Repair          *gate.Failure   `json:"repair,omitempty"`
	Review          ReviewInfo      `json:"review,omitempty"`
	Completion      *CompletionInfo `json:"completion_authority,omitempty"`
	TaskMaterial    *TaskMaterial   `json:"task_material,omitempty"`
}

// SpecSource describes the canonical Markdown contract behind status.
type SpecSource struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Bytes    int    `json:"bytes"`
	Markdown string `json:"markdown"`
}

// NextAction gives wrappers and trigger agents the next deterministic role and
// command without requiring them to interpret prose handoffs.
type NextAction struct {
	Role         string `json:"role,omitempty"`
	Action       string `json:"action,omitempty"`
	Command      string `json:"command,omitempty"`
	Reason       string `json:"reason,omitempty"`
	AfterCommand string `json:"after_command,omitempty"`
	ThenCommand  string `json:"then_command,omitempty"`
}

// ReviewInfo is the latest review evidence visible from status.
type ReviewInfo struct {
	Verdict        string                      `json:"verdict,omitempty"`
	Mode           corereview.Mode             `json:"mode,omitempty"`
	Summary        string                      `json:"summary,omitempty"`
	Findings       []corereview.Finding        `json:"findings,omitempty"`
	OpenBlockers   int                         `json:"open_blockers,omitempty"`
	AttackLog      []corereview.AttackLogEntry `json:"attack_log,omitempty"`
	Budget         corereview.Budget           `json:"budget,omitempty"`
	Provider       string                      `json:"provider,omitempty"`
	Model          string                      `json:"model,omitempty"`
	OutputFormat   string                      `json:"output_format,omitempty"`
	Normalizations []string                    `json:"normalizations,omitempty"`
	Attempt        *ReviewAttemptInfo          `json:"attempt,omitempty"`
	Running        bool                        `json:"running,omitempty"`
	AttemptStatus  string                      `json:"attempt_status,omitempty"`
	Reason         string                      `json:"reason,omitempty"`
}

// ReviewAttemptInfo describes the latest provider attempt separately from the
// latest accepted review verdict.
type ReviewAttemptInfo struct {
	AttemptID      string `json:"attempt_id,omitempty"`
	Running        bool   `json:"running"`
	Stale          bool   `json:"stale,omitempty"`
	Status         string `json:"status"`
	LeaseExpiresAt string `json:"lease_expires_at,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

// CompletionInfo describes the terminal review authority for completed tasks.
type CompletionInfo struct {
	Status        string   `json:"status"`
	Kind          string   `json:"kind"`
	Provider      string   `json:"provider,omitempty"`
	Verdict       string   `json:"verdict,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	Actual        string   `json:"actual,omitempty"`
	Summary       string   `json:"summary,omitempty"`
	ReviewEvent   string   `json:"review_event,omitempty"`
	ReceiptEvent  string   `json:"receipt_event,omitempty"`
	CompleteEvent string   `json:"complete_event,omitempty"`
	Evidence      []string `json:"evidence,omitempty"`
}

// TaskMaterial describes the task-owned workspace surface visible to status.
type TaskMaterial = reviewmaterial.Projection

// Run reads status for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string, workspaces ...WorkspaceStatus) (Output, error) {
	source, err := specsource.Load(ctx, specs, taskID)
	if err != nil {
		return Output{}, err
	}
	model := source.Model
	out := Output{
		TaskID:          model.TaskID,
		Status:          model.Status,
		Title:           model.Title,
		Next:            model.CurrentState.AllowedFollowUp,
		Gate:            currentGate(model),
		TrustedState:    "session ledger replay projected into the Markdown spec",
		AllowedFollowUp: model.CurrentState.AllowedFollowUp,
		SpecSource:      statusSpecSource(source),
	}
	reviewGateValid := false
	if sessions != nil {
		if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
			out.SessionOK = true
			workspace := firstWorkspace(workspaces)
			state := reviewgate.Project(ledger, model, reviewProjectionOptions(ctx, ledger, workspace))
			out.Review = reviewInfoFromGate(state)
			out.TaskMaterial = taskMaterialInfo(ctx, model, ledger, workspace, state.Authority)
			if model.Status == spec.StatusCompleted {
				out.Completion = completionInfo(state.Authority)
			}
			out.Repair = repairContract(model, ledger, state)
			if out.Repair != nil && out.Repair.Next != "" {
				out.Next = out.Repair.Next
				out.AllowedFollowUp = out.Repair.Next
			}
			reviewGateValid = applyReviewGateState(&out, model, state)
		}
	}
	if model.Status == spec.StatusReview && !out.SessionOK {
		out.Next = reviewCommand(model.TaskID)
		out.AllowedFollowUp = out.Next
		out.Review.Reason = "session ledger unavailable; review authority cannot be verified"
	}
	out.NextAction = nextAction(model, out.Repair, out.Review, out.Next, reviewGateValid)
	return out, nil
}

func statusSpecSource(source spec.Source) *SpecSource {
	provenance := reviewcontext.SourceForContent("file", source.Path, source.Markdown)
	return &SpecSource{
		Path:     provenance.Path,
		SHA256:   provenance.SHA256,
		Bytes:    provenance.Bytes,
		Markdown: string(source.Markdown),
	}
}

func taskMaterialInfo(ctx context.Context, model spec.Model, ledger session.Session, workspace WorkspaceStatus, authority reviewgate.Authority) *TaskMaterial {
	var current []string
	hasCurrent := false
	if workspace != nil {
		if snapshot, err := workspace.ChangedFiles(ctx); err == nil {
			current = snapshot
			hasCurrent = true
		}
	}
	currentMaterialDigest := ""
	hasCurrentMaterialDigest := false
	if workspace != nil && authority.Valid && reviewedScopePresent(authority.ReviewEntry.ReviewedScope) {
		if material, err := currentMaterialSeal(ctx, workspace, authority.ReviewEntry.ReviewedScope); err == nil {
			currentMaterialDigest = material.Digest
			hasCurrentMaterialDigest = true
		}
	}
	projection := reviewmaterial.Project(reviewmaterial.Input{
		Model:                    model,
		Ledger:                   ledger,
		CurrentSnapshot:          current,
		HasCurrentSnapshot:       hasCurrent,
		Authority:                authority,
		CurrentMaterialDigest:    currentMaterialDigest,
		HasCurrentMaterialDigest: hasCurrentMaterialDigest,
	})
	if projection.Empty() {
		return nil
	}
	return &projection
}

func firstWorkspace(workspaces []WorkspaceStatus) WorkspaceStatus {
	if len(workspaces) == 0 {
		return nil
	}
	return workspaces[0]
}

func reviewProjectionOptions(ctx context.Context, ledger session.Session, workspace WorkspaceStatus) reviewgate.Options {
	opts := reviewgate.Options{Now: time.Now().UTC()}
	if workspace == nil {
		return opts
	}
	if seal, err := currentWorkspaceSeal(ctx, workspace); err == nil {
		opts.WorkspaceSeal = seal
		opts.HasWorkspaceSeal = true
	}
	authority := reviewgate.CurrentReviewGate(ledger)
	if authority.Valid && strings.TrimSpace(authority.ReviewEntry.ReviewedMaterialDigest) != "" && reviewedScopePresent(authority.ReviewEntry.ReviewedScope) {
		if material, err := currentMaterialSeal(ctx, workspace, authority.ReviewEntry.ReviewedScope); err == nil {
			opts.MaterialSeal = material
			opts.HasMaterialSeal = true
		}
	}
	return opts
}

func currentWorkspaceSeal(ctx context.Context, workspace WorkspaceStatus) (reviewgate.WorkspaceSeal, error) {
	snapshot, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return reviewgate.WorkspaceSeal{}, err
	}
	comparison := reviewevidence.ComparisonSnapshot(snapshot)
	return reviewgate.WorkspaceSeal{
		Head:  currentHead(ctx, workspace),
		Dirty: reviewevidence.SnapshotDirty(comparison),
		Diff:  reviewevidence.SnapshotDigest(comparison),
	}, nil
}

func currentMaterialSeal(ctx context.Context, workspace WorkspaceStatus, scope []string) (reviewevidence.MaterialSeal, error) {
	material, ok := workspace.(workspaceMaterialStatus)
	if !ok {
		return reviewevidence.MaterialSeal{}, fmt.Errorf("workspace material seal is unavailable")
	}
	return material.MaterialSeal(ctx, scope)
}

func currentHead(ctx context.Context, workspace WorkspaceStatus) string {
	head, hasHead, err := workspace.ResolveHead(ctx)
	if err != nil {
		return "error:" + singleLine(err.Error())
	}
	if !hasHead || strings.TrimSpace(head) == "" {
		return "unborn"
	}
	return strings.TrimSpace(head)
}

func reviewedScopePresent(scope []string) bool {
	for _, item := range scope {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
}

func applyReviewGateState(out *Output, model spec.Model, state reviewgate.State) bool {
	if model.Status != spec.StatusReview {
		return false
	}
	if state.Kind == reviewgate.KindReviewPassed {
		out.Next = completeCommand(model.TaskID)
		out.AllowedFollowUp = out.Next
		return true
	}
	if out.Repair == nil {
		out.Next = fallback(state.Next, reviewCommand(model.TaskID))
		out.AllowedFollowUp = out.Next
		out.Review.Reason = fallback(out.Review.Reason, state.Reason)
	}
	return false
}

func nextAction(model spec.Model, repair *gate.Failure, review ReviewInfo, command string, reviewGateValid bool) NextAction {
	taskCommand := func(name string) string {
		if model.TaskID == "" {
			return "scafld " + name
		}
		return "scafld " + name + " " + model.TaskID
	}
	if command == "" {
		command = model.CurrentState.AllowedFollowUp
	}
	switch model.Status {
	case spec.StatusDraft:
		if model.HardenStatus == spec.HardenInProgress {
			return NextAction{Role: "planner", Action: "finish_hardening", Command: command, Reason: fallback(model.CurrentState.Reason, "draft hardening is in progress")}
		}
		return NextAction{Role: "operator", Action: "approve_contract", Command: command, Reason: "draft needs explicit approval before build"}
	case spec.StatusApproved:
		return NextAction{Role: "executor", Action: "open_build", Command: command, Reason: "approved contract is ready for phase execution"}
	case spec.StatusActive:
		return NextAction{Role: "executor", Action: "read_handoff", Command: command, Reason: fallback(model.CurrentState.Reason, "phase is open"), AfterCommand: taskCommand("build")}
	case spec.StatusBlocked:
		return NextAction{Role: "executor", Action: "repair_acceptance", Command: command, Reason: fallback(model.CurrentState.Reason, "acceptance is blocked"), AfterCommand: taskCommand("build")}
	case spec.StatusReview:
		if repair != nil && repair.Gate == "review" {
			if review.AttemptStatus == "stale" {
				return NextAction{Role: "reviewer", Action: "recover_stale_review_attempt", Command: command, Reason: fallback(repair.Reason, "latest review attempt is stale"), ThenCommand: taskCommand("review")}
			}
			if review.Verdict == corereview.VerdictFail {
				return NextAction{Role: "executor", Action: "repair_review_findings", Command: command, Reason: "review found completion blockers", AfterCommand: taskCommand("build"), ThenCommand: taskCommand("review")}
			}
			if repair.Next == taskCommand("review") {
				return NextAction{Role: "reviewer", Action: "run_review", Command: repair.Next, Reason: fallback(repair.Reason, "latest review gate is stale")}
			}
			return NextAction{Role: "operator", Action: "repair_review_provider", Command: command, Reason: fallback(repair.Reason, "latest review attempt failed"), ThenCommand: taskCommand("review")}
		}
		if reviewGateValid {
			return NextAction{Role: "operator", Action: "complete", Command: command, Reason: "latest review gate passed"}
		}
		return NextAction{Role: "reviewer", Action: "run_review", Command: command, Reason: fallback(review.Reason, fallback(model.CurrentState.Reason, "build completed; ready for review"))}
	case spec.StatusCompleted:
		return NextAction{Role: "none", Action: "done", Command: "none", Reason: "task is completed"}
	case spec.StatusFailed, spec.StatusCancelled:
		return NextAction{Role: "none", Action: "terminal", Command: "none", Reason: "task is terminal"}
	default:
		return NextAction{Role: "operator", Action: string(model.Status), Command: command, Reason: fallback(model.CurrentState.Reason, "inspect status")}
	}
}

func reviewCommand(taskID string) string {
	if taskID == "" {
		return "scafld review"
	}
	return "scafld review " + taskID
}

func completeCommand(taskID string) string {
	if taskID == "" {
		return "scafld complete"
	}
	return "scafld complete " + taskID
}

func completionInfo(authority reviewgate.Authority) *CompletionInfo {
	if !authority.Completed {
		return nil
	}
	return &CompletionInfo{
		Status:        authority.Status(),
		Kind:          authority.Kind(),
		Provider:      authority.Provider(),
		Verdict:       authority.Verdict(),
		Reason:        authority.Reason,
		Actual:        authority.Actual,
		Summary:       authority.Summary(),
		ReviewEvent:   eventReference(authority.ReviewEntry),
		ReceiptEvent:  eventReference(authority.ReceiptEntry),
		CompleteEvent: eventReference(authority.CompleteEntry),
		Evidence:      authority.Evidence,
	}
}

func reviewInfoFromGate(state reviewgate.State) ReviewInfo {
	var info ReviewInfo
	if state.HasAttempt {
		status := state.LatestAttempt.Status
		if state.LatestAttempt.Stale {
			status = "stale"
		}
		info.Running = state.LatestAttempt.Running && !state.LatestAttempt.Stale
		info.AttemptStatus = status
		info.Reason = fallback(state.Reason, state.LatestAttempt.Reason)
		leaseExpiresAt := ""
		if state.LatestAttempt.HasLease {
			leaseExpiresAt = state.LatestAttempt.LeaseExpiresAt.Format(time.RFC3339Nano)
		}
		info.Attempt = &ReviewAttemptInfo{
			AttemptID:      state.LatestAttempt.AttemptID,
			Running:        info.Running,
			Stale:          state.LatestAttempt.Stale,
			Status:         status,
			LeaseExpiresAt: leaseExpiresAt,
			Reason:         state.LatestAttempt.Reason,
		}
	}
	if !state.HasReview {
		return info
	}
	entry := state.LatestReview
	info.Verdict = entry.Status
	info.Provider = entry.Provider
	info.Summary = entry.Reason
	if state.HasDossier {
		dossier := state.Dossier
		info.Mode = dossier.Mode
		info.Summary = dossier.Summary
		info.Findings = dossier.Findings
		info.OpenBlockers = corereview.OpenBlockerCount(dossier.Findings)
		info.AttackLog = dossier.AttackLog
		info.Budget = dossier.Budget
		info.Provider = dossier.Provider
		info.Model = dossier.Model
		info.OutputFormat = dossier.OutputFormat
		info.Normalizations = dossier.Normalizations
	}
	return info
}

func currentGate(model spec.Model) string {
	switch model.Status {
	case spec.StatusDraft:
		if model.HardenStatus == spec.HardenInProgress {
			return "harden"
		}
		return "approve"
	case spec.StatusApproved, spec.StatusActive, spec.StatusBlocked:
		return "build"
	case spec.StatusReview:
		return "review"
	case spec.StatusCompleted:
		return "complete"
	default:
		return string(model.Status)
	}
}

func repairContract(model spec.Model, ledger session.Session, state reviewgate.State) *gate.Failure {
	switch model.Status {
	case spec.StatusBlocked:
		blockers := criterionBlockers(model)
		return &gate.Failure{
			Gate:     "build",
			Status:   string(model.Status),
			Reason:   model.CurrentState.Reason,
			Evidence: blockerEvidence(model, ledger),
			Expected: "all acceptance criteria pass",
			Actual:   fmt.Sprintf("%d blocker(s)", len(blockers)),
			Blockers: blockers,
			Next:     model.CurrentState.AllowedFollowUp,
		}
	case spec.StatusReview:
		review := reviewInfoFromGate(state)
		if state.Kind == reviewgate.KindAttemptFailed {
			evidence := nonEmptyStrings(state.Evidence, "latest review_attempt session entry")
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   fallback(state.Reason, "latest review attempt failed"),
				Evidence: evidence,
				Expected: "valid ReviewDossier submitted by an external reviewer",
				Actual:   fallback(state.Actual, "review attempt failed"),
				Blockers: []string{fallback(state.Reason, "latest review attempt failed")},
				Next:     "scafld handoff " + model.TaskID,
			}
		}
		if state.Kind == reviewgate.KindAttemptStale {
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   state.Reason,
				Evidence: nonEmptyStrings(state.Evidence, "latest review_attempt session entry"),
				Expected: "no stale running review attempt before review starts",
				Actual:   fallback(state.Actual, "review attempt lease expired"),
				Blockers: []string{"run scafld review " + model.TaskID + " to abandon the stale attempt and start a new leased attempt"},
				Next:     "scafld review " + model.TaskID,
			}
		}
		if state.Kind == reviewgate.KindReviewFailed {
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   "review verdict fail",
				Evidence: nonEmptyStrings(state.Evidence, latestReviewEvidence(ledger)),
				Expected: "review verdict pass",
				Actual:   "review verdict fail",
				Blockers: reviewFindingSummaries(review.Findings),
				Next:     model.CurrentState.AllowedFollowUp,
			}
		}
		if reviewStaleKind(state.Kind) || state.Kind == reviewgate.KindInvalid {
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   fallback(state.Reason, "latest review gate has not passed"),
				Evidence: nonEmptyStrings(state.Evidence, latestReviewEvidence(ledger)),
				Expected: "current passing review evidence",
				Actual:   fallback(state.Actual, "review authority is stale"),
				Blockers: []string{fallback(state.Reason, "latest review gate has not passed")},
				Next:     fallback(state.Next, "scafld review "+model.TaskID),
			}
		}
	}
	return nil
}

func reviewStaleKind(kind reviewgate.Kind) bool {
	switch kind {
	case reviewgate.KindReviewStaleAfterBuild, reviewgate.KindReviewStaleAfterSpec, reviewgate.KindReviewStaleAfterWorkspace:
		return true
	default:
		return false
	}
}

func fallback(value string, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
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

func nonEmptyStrings(values []string, fallback string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 && fallback != "" {
		return []string{fallback}
	}
	return out
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func eventReference(entry session.Entry) string {
	if entry.ID != "" {
		return entry.ID
	}
	if entry.Type != "" {
		return entry.Type + " session entry"
	}
	return ""
}

func criterionBlockers(model spec.Model) []string {
	var blockers []string
	for _, criterion := range blockingCriteria(model) {
		if criterion.Status == "pass" {
			continue
		}
		label := criterion.ID
		if criterion.Title != "" {
			label += ": " + criterion.Title
		}
		if criterion.Evidence != "" {
			label += " (" + criterion.Evidence + ")"
		}
		blockers = append(blockers, label)
	}
	return blockers
}

func blockerEvidence(model spec.Model, ledger session.Session) []string {
	var evidence []string
	for _, criterion := range blockingCriteria(model) {
		if criterion.Status == "pass" || criterion.SourceEvent == "" {
			continue
		}
		evidence = append(evidence, evidenceReference(ledger, criterion.SourceEvent))
	}
	return evidence
}

func evidenceReference(ledger session.Session, sourceID string) string {
	for _, entry := range ledger.Entries {
		if entry.ID != sourceID {
			continue
		}
		if entry.Path != "" {
			return entry.Path
		}
		return sourceID
	}
	return sourceID
}

func blockingCriteria(model spec.Model) []spec.Criterion {
	switch model.CurrentState.CurrentPhase {
	case "", "none":
		return model.AllCriteria()
	case "final":
		return append([]spec.Criterion(nil), model.Acceptance.Criteria...)
	default:
		for _, phase := range model.Phases {
			if phase.ID == model.CurrentState.CurrentPhase {
				criteria := append([]spec.Criterion(nil), phase.Acceptance...)
				for i := range criteria {
					if criteria[i].PhaseID == "" {
						criteria[i].PhaseID = phase.ID
					}
				}
				return criteria
			}
		}
		return model.AllCriteria()
	}
}

func reviewFindingSummaries(findings []corereview.Finding) []string {
	blockers := make([]string, 0, len(findings))
	for _, finding := range findings {
		if !corereview.BlocksCompletion(finding) {
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
	return blockers
}
