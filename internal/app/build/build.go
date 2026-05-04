package build

import (
	"context"
	"fmt"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

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

// Clock supplies build timestamps.
type Clock interface{ Now() time.Time }

// Input describes the task and working directory to build.
type Input struct {
	TaskID string
	CWD    string
}

// Output summarizes acceptance execution.
type Output struct {
	TaskID string      `json:"task_id"`
	Status spec.Status `json:"status"`
	Passed int         `json:"passed"`
	Failed int         `json:"failed"`
}

// Run executes acceptance criteria and projects evidence into the spec.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, runner Runner, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	model.Status = spec.StatusActive
	now := clock.Now().UTC().Format(time.RFC3339)
	if err := runCriteria(ctx, sessions, runner, model, input.CWD, now); err != nil {
		return Output{}, err
	}
	ledger, _ := sessions.Load(ctx, model.TaskID)
	ledger, err = appendPhaseEvidence(ctx, sessions, model, ledger, now)
	if err != nil {
		return Output{}, err
	}
	replayed := session.Replay(ledger)
	passed, failed := countCriterionStates(replayed)
	applyPostBuildState(&model, failed)
	model.Updated = now
	model = reconcile.FromSession(model, ledger)
	model.Status = mapStatus(failed)
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save projected spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Passed: passed, Failed: failed}, nil
}

func runCriteria(ctx context.Context, sessions SessionStore, runner Runner, model spec.Model, cwd string, now string) error {
	for _, criterion := range model.AllCriteria() {
		if criterion.Command == "" {
			continue
		}
		entry := criterionEntry(ctx, runner, criterion, cwd)
		if _, err := sessions.Append(ctx, model.TaskID, entry, now); err != nil {
			return fmt.Errorf("append criterion evidence: %w", err)
		}
	}
	return nil
}

func criterionEntry(ctx context.Context, runner Runner, criterion spec.Criterion, cwd string) session.Entry {
	result, runErr := runner.Run(ctx, execution.Request{Command: criterion.Command, CWD: cwd, Timeout: 30 * time.Second})
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

func countCriterionStates(replayed session.Session) (int, int) {
	passed, failed := 0, 0
	for _, state := range replayed.CriterionStates {
		switch state.Status {
		case "pass":
			passed++
		case "fail", "invalid":
			failed++
		}
	}
	return passed, failed
}

func applyPostBuildState(model *spec.Model, failed int) {
	if failed > 0 {
		model.Status = spec.StatusBlocked
		model.CurrentState.Next = "fail or repair"
		model.CurrentState.AllowedFollowUp = "scafld status " + model.TaskID
		return
	}
	model.Status = spec.StatusReview
	model.CurrentState.Next = "review"
	model.CurrentState.AllowedFollowUp = "scafld review " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func mapStatus(failed int) spec.Status {
	if failed > 0 {
		return spec.StatusBlocked
	}
	return spec.StatusReview
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
