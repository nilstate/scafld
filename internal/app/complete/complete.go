package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrReviewGate is returned when completion is attempted before a passing review.
var ErrReviewGate = errors.New("review gate has not passed")

// SpecStore is the spec persistence port used by completion.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by completion.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Clock supplies completion timestamps.
type Clock interface{ Now() time.Time }

// Run completes a reviewed task and records completion evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, taskID string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return spec.Model{}, reviewGateError(model, "session ledger could not be loaded", "missing review evidence")
	}
	authority := corecompletion.CurrentReviewGate(ledger)
	if !authority.Valid {
		return spec.Model{}, reviewGateError(model, authority.Reason, authority.Actual)
	}
	model = reconcile.FromSession(model, ledger)
	if model.Status != spec.StatusReview || model.Review.Verdict != corereview.VerdictPass {
		return spec.Model{}, reviewGateError(model, "projected spec is not at a passing review gate", fmt.Sprintf("status %s verdict %s", model.Status, model.Review.Verdict))
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	ledger, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "complete", Status: "completed", Reason: "task completed"}, now)
	if err != nil {
		return spec.Model{}, err
	}
	if loaded, loadErr := sessions.Load(ctx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.Status = spec.StatusCompleted
	model.Updated = now
	model.CurrentState.Next = "done"
	model.CurrentState.AllowedFollowUp = "none"
	if err := specs.Save(ctx, path, model); err != nil {
		return spec.Model{}, err
	}
	return model, nil
}

func reviewGateError(model spec.Model, reason string, actual string) error {
	next := model.CurrentState.AllowedFollowUp
	if next == "scafld complete "+model.TaskID {
		next = "scafld handoff " + model.TaskID
	}
	if next == "" || next == "none" {
		next = "scafld review " + model.TaskID
	}
	return gate.New(ErrReviewGate, gate.Failure{
		Gate:     "complete",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: []string{"session review entries", "projected spec review state"},
		Expected: "latest accepted review verdict pass from codex, claude, gemini, command, or audited human provider",
		Actual:   actual,
		Blockers: []string{reason},
		Next:     next,
	})
}
