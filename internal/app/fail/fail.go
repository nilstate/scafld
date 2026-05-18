package fail

import (
	"context"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec persistence port used by failure.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by failure.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
}

// Clock supplies failure timestamps.
type Clock interface{ Now() time.Time }

// Run marks a task failed and records failure evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, taskID string, reason string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if _, err := sessions.Append(ctx, model.TaskID, session.Entry{Type: "fail", Status: "failed", Reason: reason}, now); err != nil {
		return spec.Model{}, err
	}
	model.Status = spec.StatusFailed
	model.Updated = now
	model.CurrentState.Reason = reason
	model.CurrentState.Next = "inspect failure"
	model.CurrentState.Blockers = reason
	model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
	if err := specs.Save(ctx, path, model); err != nil {
		return spec.Model{}, err
	}
	return model, nil
}
