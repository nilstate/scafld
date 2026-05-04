package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	if model.Review.Status != "completed" || model.Review.Verdict != corereview.VerdictPass {
		return spec.Model{}, fmt.Errorf("%w: run scafld review %s first", ErrReviewGate, model.TaskID)
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	model.Status = spec.StatusCompleted
	model.Updated = now
	model.CurrentState.Next = "done"
	model.CurrentState.AllowedFollowUp = "none"
	ledger, err := sessions.Append(ctx, model.TaskID, session.Entry{Type: "complete", Status: "completed", Reason: "task completed"}, now)
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
