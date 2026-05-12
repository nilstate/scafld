package build

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrSpecNotBuildable is returned when execution is attempted before approval or after terminal state.
var ErrSpecNotBuildable = errors.New("build requires approved, active, blocked, or failed-review state")

const finalPhaseID = "final"

// SpecStore is the spec persistence port used by build.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by build.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Runner executes acceptance commands.
type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

// WorkspaceStatus captures dirty workspace state when older sessions lack a task baseline.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies build timestamps.
type Clock interface{ Now() time.Time }

// Input describes the task and working directory to build.
type Input struct {
	TaskID string
	CWD    string
	Env    []string
}

// Output summarizes one build lifecycle step.
type Output struct {
	TaskID string        `json:"task_id"`
	Status spec.Status   `json:"status"`
	Phase  string        `json:"phase,omitempty"`
	Passed int           `json:"passed"`
	Failed int           `json:"failed"`
	Next   string        `json:"next"`
	Repair *gate.Failure `json:"repair,omitempty"`
}

// GateFailure exposes blocked build repair details to the CLI envelope.
func (o Output) GateFailure() *gate.Failure { return o.Repair }

// Run advances the governed execution lifecycle. The first build call opens the
// first phase without running future acceptance. Later calls run evidence for
// the current phase, open the next phase, or move the task to review.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, runner Runner, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	if !buildable(model.Status) {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotBuildable, model.Status)
	}
	if model.Status == spec.StatusReview && model.CurrentState.Next != "repair" {
		return Output{}, fmt.Errorf("%w: %s is ready for review", ErrSpecNotBuildable, model.Status)
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if err := ensureWorkspaceBaseline(ctx, sessions, workspace, model, now); err != nil {
		return Output{}, err
	}
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return Output{}, fmt.Errorf("load session evidence: %w", err)
	}
	if model.Status == spec.StatusApproved {
		return openBuild(ctx, specs, sessions, model, path, ledger, now)
	}
	return continueBuild(ctx, specs, sessions, runner, model, path, ledger, input.CWD, input.Env, now)
}

func openBuild(ctx context.Context, specs SpecStore, sessions SessionStore, model spec.Model, path string, ledger session.Session, now string) (Output, error) {
	phaseID, ok := firstPendingPhase(model)
	if !ok {
		ledger, err := appendBuild(ctx, sessions, model.TaskID, spec.StatusActive, "final acceptance opened", now)
		if err != nil {
			return Output{}, err
		}
		model = reconcile.FromSession(model, ledger)
		model.Updated = now
		applyActiveState(&model, finalPhaseID, "final acceptance opened")
		if err := specs.Save(ctx, path, model); err != nil {
			return Output{}, fmt.Errorf("save projected spec: %w", err)
		}
		return Output{TaskID: model.TaskID, Status: model.Status, Phase: finalPhaseID, Next: model.CurrentState.AllowedFollowUp}, nil
	}
	var err error
	ledger, err = appendPhase(ctx, sessions, model.TaskID, phaseID, "active", "phase "+phaseID+" opened", now)
	if err != nil {
		return Output{}, err
	}
	ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusActive, "phase "+phaseID+" opened", now)
	if err != nil {
		return Output{}, err
	}
	model = reconcile.FromSession(model, ledger)
	model.Updated = now
	applyActiveState(&model, phaseID, "phase "+phaseID+" opened")
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save projected spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Phase: phaseID, Next: model.CurrentState.AllowedFollowUp}, nil
}

func continueBuild(ctx context.Context, specs SpecStore, sessions SessionStore, runner Runner, model spec.Model, path string, ledger session.Session, cwd string, env []string, now string) (Output, error) {
	phaseID := currentPhaseID(model)
	if phaseID == "" {
		phaseID = finalPhaseID
	}
	if phaseID == finalPhaseID {
		return runFinalAcceptance(ctx, specs, sessions, runner, model, path, ledger, cwd, env, now, 0, 0)
	}
	phase, ok := phaseByID(model, phaseID)
	if !ok {
		return Output{}, fmt.Errorf("%w: current phase %s is not declared", ErrSpecNotBuildable, phaseID)
	}
	var err error
	ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusActive, "running phase "+phaseID+" evidence", now)
	if err != nil {
		return Output{}, err
	}
	ledger, passed, failed, err := runCriterionList(ctx, sessions, runner, model.TaskID, phase.Acceptance, phaseID, cwd, env, now)
	if err != nil {
		return Output{}, err
	}
	if failed > 0 {
		ledger, err = appendPhase(ctx, sessions, model.TaskID, phaseID, "blocked", "phase "+phaseID+" acceptance failed", now)
		if err != nil {
			return Output{}, err
		}
		ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusBlocked, "phase "+phaseID+" acceptance failed", now)
		if err != nil {
			return Output{}, err
		}
		model = reconcile.FromSession(model, ledger)
		model.Updated = now
		applyBlockedState(&model, phaseID, "phase "+phaseID+" acceptance failed")
		if err := specs.Save(ctx, path, model); err != nil {
			return Output{}, fmt.Errorf("save projected spec: %w", err)
		}
		return Output{TaskID: model.TaskID, Status: model.Status, Phase: phaseID, Passed: passed, Failed: failed, Next: model.CurrentState.AllowedFollowUp, Repair: buildRepair(model, ledger)}, nil
	}
	ledger, err = appendPhase(ctx, sessions, model.TaskID, phaseID, "completed", "all phase criteria passed", now)
	if err != nil {
		return Output{}, err
	}
	model = reconcile.FromSession(model, ledger)
	if nextPhase, ok := firstPendingPhase(model); ok {
		ledger, err = appendPhase(ctx, sessions, model.TaskID, nextPhase, "active", "phase "+nextPhase+" opened", now)
		if err != nil {
			return Output{}, err
		}
		ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusActive, "phase "+phaseID+" completed; phase "+nextPhase+" opened", now)
		if err != nil {
			return Output{}, err
		}
		model = reconcile.FromSession(model, ledger)
		model.Updated = now
		applyActiveState(&model, nextPhase, "phase "+phaseID+" completed; phase "+nextPhase+" opened")
		if err := specs.Save(ctx, path, model); err != nil {
			return Output{}, fmt.Errorf("save projected spec: %w", err)
		}
		return Output{TaskID: model.TaskID, Status: model.Status, Phase: nextPhase, Passed: passed, Failed: failed, Next: model.CurrentState.AllowedFollowUp}, nil
	}
	return runFinalAcceptance(ctx, specs, sessions, runner, model, path, ledger, cwd, env, now, passed, failed)
}

func runFinalAcceptance(ctx context.Context, specs SpecStore, sessions SessionStore, runner Runner, model spec.Model, path string, ledger session.Session, cwd string, env []string, now string, passed int, failed int) (Output, error) {
	if len(model.Acceptance.Criteria) > 0 {
		var err error
		ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusActive, "running final acceptance", now)
		if err != nil {
			return Output{}, err
		}
		var finalPassed, finalFailed int
		ledger, finalPassed, finalFailed, err = runCriterionList(ctx, sessions, runner, model.TaskID, model.Acceptance.Criteria, "", cwd, env, now)
		if err != nil {
			return Output{}, err
		}
		passed += finalPassed
		failed += finalFailed
	}
	if failed > 0 {
		var err error
		ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusBlocked, "final acceptance failed", now)
		if err != nil {
			return Output{}, err
		}
		model = reconcile.FromSession(model, ledger)
		model.Updated = now
		applyBlockedState(&model, finalPhaseID, "final acceptance failed")
		if err := specs.Save(ctx, path, model); err != nil {
			return Output{}, fmt.Errorf("save projected spec: %w", err)
		}
		return Output{TaskID: model.TaskID, Status: model.Status, Phase: finalPhaseID, Passed: passed, Failed: failed, Next: model.CurrentState.AllowedFollowUp, Repair: buildRepair(model, ledger)}, nil
	}
	var err error
	ledger, err = appendBuild(ctx, sessions, model.TaskID, spec.StatusReview, "build completed; ready for review", now)
	if err != nil {
		return Output{}, err
	}
	model = reconcile.FromSession(model, ledger)
	model.Updated = now
	applyReviewState(&model)
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save projected spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Phase: finalPhaseID, Passed: passed, Failed: failed, Next: model.CurrentState.AllowedFollowUp}, nil
}

func ensureWorkspaceBaseline(ctx context.Context, sessions SessionStore, workspace WorkspaceStatus, model spec.Model, now string) error {
	if sessions == nil || workspace == nil {
		return nil
	}
	if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
		if _, ok := session.FirstWorkspaceBaseline(ledger); ok {
			return nil
		}
	}
	snapshot, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return fmt.Errorf("capture workspace baseline: %w", err)
	}
	_, err = sessions.Append(ctx, model.TaskID, session.Entry{
		Type:   session.EntryWorkspaceBaseline,
		Status: "captured",
		Reason: fmt.Sprintf("workspace baseline captured before build: %d changed path(s)", len(snapshot)),
		Output: strings.Join(snapshot, "\n"),
	}, now)
	if err != nil {
		return fmt.Errorf("append workspace baseline: %w", err)
	}
	return nil
}

func buildable(status spec.Status) bool {
	switch status {
	case spec.StatusApproved, spec.StatusActive, spec.StatusBlocked, spec.StatusReview:
		return true
	default:
		return false
	}
}

func runCriterionList(ctx context.Context, sessions SessionStore, runner Runner, taskID string, criteria []spec.Criterion, phaseID string, cwd string, env []string, now string) (session.Session, int, int, error) {
	var ledger session.Session
	passed, failed := 0, 0
	for _, criterion := range criteria {
		if criterion.PhaseID == "" {
			criterion.PhaseID = phaseID
		}
		entry := criterionEntry(ctx, runner, criterion, cwd, env)
		var err error
		ledger, err = sessions.Append(ctx, taskID, entry, now)
		if err != nil {
			return ledger, passed, failed, fmt.Errorf("append criterion evidence: %w", err)
		}
		if entry.Status == "pass" {
			passed++
		} else {
			failed++
		}
	}
	if len(criteria) == 0 {
		var err error
		ledger, err = sessions.Load(ctx, taskID)
		if err != nil {
			return ledger, passed, failed, fmt.Errorf("load session evidence: %w", err)
		}
	}
	return ledger, passed, failed, nil
}

func criterionEntry(ctx context.Context, runner Runner, criterion spec.Criterion, cwd string, env []string) session.Entry {
	var result execution.Result
	var runErr error
	if strings.TrimSpace(criterion.Command) == "" {
		evaluation := acceptance.Evaluate(criterion.ExpectedKind, acceptance.Evidence{})
		return session.Entry{
			Type:        "criterion",
			CriterionID: criterion.ID,
			PhaseID:     criterion.PhaseID,
			Status:      evaluation.Status,
			Reason:      evaluation.Reason,
		}
	}
	if runner != nil {
		result, runErr = runner.Run(ctx, execution.Request{Command: criterion.Command, CWD: cwd, Env: env, Timeout: 30 * time.Second})
	} else {
		runErr = errors.New("missing acceptance runner")
	}
	evaluation := acceptance.Evaluate(criterion.ExpectedKind, acceptance.Evidence{ExitCode: result.ExitCode, Output: result.Output})
	if runErr != nil {
		evaluation.Status = "fail"
		evaluation.Reason = runErr.Error()
	}
	return session.Entry{
		Type:        "criterion",
		CriterionID: criterion.ID,
		PhaseID:     criterion.PhaseID,
		Status:      evaluation.Status,
		Reason:      evaluation.Reason,
		Command:     criterion.Command,
		ExitCode:    result.ExitCode,
		Output:      snippet(result.Output),
		Path:        result.DiagnosticPath,
	}
}

func appendPhase(ctx context.Context, sessions SessionStore, taskID string, phaseID string, status string, reason string, now string) (session.Session, error) {
	ledger, err := sessions.Append(ctx, taskID, session.Entry{Type: "phase", PhaseID: phaseID, Status: status, Reason: reason}, now)
	if err != nil {
		return ledger, fmt.Errorf("append phase evidence: %w", err)
	}
	return ledger, nil
}

func appendBuild(ctx context.Context, sessions SessionStore, taskID string, status spec.Status, reason string, now string) (session.Session, error) {
	ledger, err := sessions.Append(ctx, taskID, session.Entry{Type: "build", Status: string(status), Reason: reason}, now)
	if err != nil {
		return ledger, fmt.Errorf("append build state: %w", err)
	}
	return ledger, nil
}

func applyActiveState(model *spec.Model, phaseID string, reason string) {
	model.Status = spec.StatusActive
	model.CurrentState.CurrentPhase = phaseID
	model.CurrentState.Next = "build"
	model.CurrentState.Reason = reason
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func applyBlockedState(model *spec.Model, phaseID string, reason string) {
	model.Status = spec.StatusBlocked
	model.CurrentState.CurrentPhase = phaseID
	model.CurrentState.Next = "repair"
	model.CurrentState.Reason = reason
	model.CurrentState.Blockers = reason
	model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func applyReviewState(model *spec.Model) {
	model.Status = spec.StatusReview
	model.CurrentState.CurrentPhase = finalPhaseID
	model.CurrentState.Next = "review"
	model.CurrentState.Reason = "build completed; ready for review"
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld review " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func buildRepair(model spec.Model, ledger session.Session) *gate.Failure {
	if model.Status != spec.StatusBlocked {
		return nil
	}
	currentPhase := strings.TrimSpace(model.CurrentState.CurrentPhase)
	var blockers []string
	var evidence []string
	for _, criterion := range model.AllCriteria() {
		if currentPhase != "" && currentPhase != finalPhaseID && criterion.PhaseID != currentPhase {
			continue
		}
		if currentPhase == finalPhaseID && criterion.PhaseID != "" {
			continue
		}
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
		if ref := evidenceReference(ledger, criterion.SourceEvent); ref != "" {
			evidence = append(evidence, ref)
		}
	}
	return &gate.Failure{
		Gate:     "build",
		Status:   string(model.Status),
		Reason:   "acceptance criteria failed",
		Evidence: evidence,
		Expected: "all acceptance criteria pass",
		Actual:   fmt.Sprintf("%d blocker(s)", len(blockers)),
		Blockers: blockers,
		Next:     model.CurrentState.AllowedFollowUp,
	}
}

func evidenceReference(ledger session.Session, sourceID string) string {
	if sourceID == "" {
		return ""
	}
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

func snippet(s string) string {
	if len(s) > 1000 {
		return s[:1000]
	}
	return s
}

func firstPendingPhase(model spec.Model) (string, bool) {
	for _, phase := range model.Phases {
		switch phase.Status {
		case "completed":
			continue
		default:
			return phase.ID, true
		}
	}
	return "", false
}

func currentPhaseID(model spec.Model) string {
	current := strings.TrimSpace(model.CurrentState.CurrentPhase)
	if current == finalPhaseID {
		return finalPhaseID
	}
	if current != "" && current != "none" {
		if phase, ok := phaseByID(model, current); ok && phase.Status != "completed" {
			return current
		}
	}
	for _, phase := range model.Phases {
		if phase.Status == "active" || phase.Status == "blocked" {
			return phase.ID
		}
	}
	if phaseID, ok := firstPendingPhase(model); ok {
		return phaseID
	}
	return finalPhaseID
}

func phaseByID(model spec.Model, phaseID string) (spec.Phase, bool) {
	for _, phase := range model.Phases {
		if phase.ID == phaseID {
			return phase, true
		}
	}
	return spec.Phase{}, false
}
