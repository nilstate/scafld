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

type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

type Clock interface{ Now() time.Time }

type Input struct {
	TaskID string
	CWD    string
}

type Output struct {
	TaskID string
	Status spec.Status
	Passed int
	Failed int
}

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, runner Runner, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	model.Status = spec.StatusActive
	var ledger session.Session
	now := clock.Now().UTC().Format(time.RFC3339)
	for _, criterion := range model.AllCriteria() {
		if criterion.Command == "" {
			continue
		}
		result, runErr := runner.Run(ctx, execution.Request{Command: criterion.Command, CWD: input.CWD, Timeout: 30 * time.Second})
		evaluation := acceptance.Evaluate(criterion.ExpectedKind, acceptance.Evidence{ExitCode: result.ExitCode, Output: result.Output})
		if runErr != nil && evaluation.Status == "pass" {
			evaluation.Status = "fail"
			evaluation.Reason = runErr.Error()
		}
		entry := session.Entry{
			Type:        "criterion",
			CriterionID: criterion.ID,
			PhaseID:     criterion.PhaseID,
			Status:      evaluation.Status,
			Reason:      evaluation.Reason,
			Command:     criterion.Command,
			ExitCode:    result.ExitCode,
			Output:      snippet(result.Output),
		}
		ledger, err = sessions.Append(ctx, model.TaskID, entry, now)
		if err != nil {
			return Output{}, fmt.Errorf("append criterion evidence: %w", err)
		}
	}
	ledger, _ = sessions.Load(ctx, model.TaskID)
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
			return Output{}, fmt.Errorf("append phase evidence: %w", err)
		}
	}
	replayed := session.Replay(ledger)
	passed, failed := 0, 0
	for _, state := range replayed.CriterionStates {
		if state.Status == "pass" {
			passed++
		}
		if state.Status == "fail" || state.Status == "invalid" {
			failed++
		}
	}
	if failed > 0 {
		model.Status = spec.StatusBlocked
		model.CurrentState.Next = "fail or repair"
		model.CurrentState.AllowedFollowUp = "scafld status " + model.TaskID
	} else {
		model.Status = spec.StatusReview
		model.CurrentState.Next = "review"
		model.CurrentState.AllowedFollowUp = "scafld review " + model.TaskID
		model.CurrentState.ReviewGate = "not_started"
	}
	model.Updated = now
	model = reconcile.FromSession(model, ledger)
	model.Status = mapStatus(failed)
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save projected spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Passed: passed, Failed: failed}, nil
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
