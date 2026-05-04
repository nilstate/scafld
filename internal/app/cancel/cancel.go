package cancel

import (
	"context"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
}

type Clock interface{ Now() time.Time }

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, taskID string, reason string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	model.Status = spec.StatusCancelled
	model.Updated = now
	model.CurrentState.Reason = reason
	model.CurrentState.Next = "done"
	if err := specs.Save(ctx, path, model); err != nil {
		return spec.Model{}, err
	}
	_, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "cancel", Status: "cancelled", Reason: reason}, now)
	return model, err
}
