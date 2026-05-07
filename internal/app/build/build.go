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
var ErrSpecNotBuildable = errors.New("build requires approved, active, blocked, or review state")

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

// Output summarizes acceptance execution.
type Output struct {
	TaskID string        `json:"task_id"`
	Status spec.Status   `json:"status"`
	Passed int           `json:"passed"`
	Failed int           `json:"failed"`
	Next   string        `json:"next"`
	Repair *gate.Failure `json:"repair,omitempty"`
}

// GateFailure exposes blocked build repair details to the CLI envelope.
func (o Output) GateFailure() *gate.Failure { return o.Repair }

// Run executes acceptance criteria and projects evidence into the spec.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, runner Runner, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	if !buildable(model.Status) {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotBuildable, model.Status)
	}
	model.Status = spec.StatusActive
	now := clock.Now().UTC().Format(time.RFC3339)
	if err := ensureWorkspaceBaseline(ctx, sessions, workspace, model, now); err != nil {
		return Output{}, err
	}
	if _, err := sessions.Append(ctx, model.TaskID, session.Entry{Type: "build", Status: "active", Reason: "build started"}, now); err != nil {
		return Output{}, fmt.Errorf("append build start: %w", err)
	}
	if err := runCriteria(ctx, sessions, runner, model, input.CWD, input.Env, now); err != nil {
		return Output{}, err
	}
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return Output{}, fmt.Errorf("load session evidence: %w", err)
	}
	ledger, err = appendPhaseEvidence(ctx, sessions, model, ledger, now)
	if err != nil {
		return Output{}, err
	}
	replayed := session.Replay(ledger)
	passed, failed := countCriterionOutcomes(model, replayed)
	applyPostBuildState(&model, failed)
	model.Updated = now
	ledger, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "build", Status: string(model.Status), Reason: "build completed"}, now)
	if err != nil {
		return Output{}, fmt.Errorf("append build result: %w", err)
	}
	model = reconcile.FromSession(model, ledger)
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save projected spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Passed: passed, Failed: failed, Next: model.CurrentState.AllowedFollowUp, Repair: buildRepair(model, ledger)}, nil
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

func runCriteria(ctx context.Context, sessions SessionStore, runner Runner, model spec.Model, cwd string, env []string, now string) error {
	for _, phase := range model.Phases {
		phaseBlocked := false
		for _, criterion := range phase.Acceptance {
			if criterion.PhaseID == "" {
				criterion.PhaseID = phase.ID
			}
			if criterion.Command == "" {
				if criterion.Status != "pass" {
					phaseBlocked = true
				}
				continue
			}
			entry := criterionEntry(ctx, runner, criterion, cwd, env)
			if _, err := sessions.Append(ctx, model.TaskID, entry, now); err != nil {
				return fmt.Errorf("append criterion evidence: %w", err)
			}
			if entry.Status != "pass" {
				phaseBlocked = true
			}
		}
		if phaseBlocked {
			break
		}
	}
	for _, criterion := range model.Acceptance.Criteria {
		if criterion.Command == "" {
			continue
		}
		entry := criterionEntry(ctx, runner, criterion, cwd, env)
		if _, err := sessions.Append(ctx, model.TaskID, entry, now); err != nil {
			return fmt.Errorf("append criterion evidence: %w", err)
		}
	}
	return nil
}

func criterionEntry(ctx context.Context, runner Runner, criterion spec.Criterion, cwd string, env []string) session.Entry {
	result, runErr := runner.Run(ctx, execution.Request{Command: criterion.Command, CWD: cwd, Env: env, Timeout: 30 * time.Second})
	evaluation := acceptance.Evaluate(criterion.ExpectedKind, acceptance.Evidence{ExitCode: result.ExitCode, Output: result.Output})
	if runErr != nil && evaluation.Status == "pass" {
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

func appendPhaseEvidence(ctx context.Context, sessions SessionStore, model spec.Model, ledger session.Session, now string) (session.Session, error) {
	var err error
	for _, phase := range model.Phases {
		status, reason := phaseEvidenceState(phase, ledger)
		if status == "" {
			continue
		}
		ledger, err = sessions.Append(ctx, model.TaskID, session.Entry{
			Type:    "phase",
			PhaseID: phase.ID,
			Status:  status,
			Reason:  reason,
		}, now)
		if err != nil {
			return ledger, fmt.Errorf("append phase evidence: %w", err)
		}
	}
	return ledger, nil
}

func countCriterionOutcomes(model spec.Model, replayed session.Session) (int, int) {
	passed, failed := 0, 0
	for _, criterion := range model.AllCriteria() {
		state, ok := replayed.CriterionStates[criterion.ID]
		if !ok {
			failed++
			continue
		}
		switch state.Status {
		case "pass":
			passed++
		default:
			failed++
		}
	}
	return passed, failed
}

func applyPostBuildState(model *spec.Model, failed int) {
	if failed > 0 {
		model.Status = spec.StatusBlocked
		model.CurrentState.Next = "repair"
		model.CurrentState.Reason = "acceptance criteria failed"
		model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
		model.CurrentState.ReviewGate = "not_started"
		return
	}
	model.Status = spec.StatusReview
	model.CurrentState.Next = "review"
	model.CurrentState.AllowedFollowUp = "scafld review " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func buildRepair(model spec.Model, ledger session.Session) *gate.Failure {
	if model.Status != spec.StatusBlocked {
		return nil
	}
	var blockers []string
	var evidence []string
	for _, criterion := range model.AllCriteria() {
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

func phaseEvidenceState(phase spec.Phase, ledger session.Session) (string, string) {
	if len(phase.Acceptance) == 0 {
		return "", ""
	}
	allPass := true
	for _, criterion := range phase.Acceptance {
		state, ok := ledger.CriterionStates[criterion.ID]
		if !ok {
			return "", ""
		}
		if state.Status != "pass" {
			allPass = false
		}
	}
	if allPass {
		return "completed", "all phase criteria passed"
	}
	return "blocked", "one or more phase criteria failed"
}
