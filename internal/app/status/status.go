package status

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

type Output struct {
	TaskID    string
	Status    spec.Status
	Title     string
	Next      string
	SessionOK bool
}

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (Output, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	out := Output{TaskID: model.TaskID, Status: model.Status, Title: model.Title, Next: model.CurrentState.AllowedFollowUp}
	if sessions != nil {
		if _, err := sessions.Load(ctx, model.TaskID); err == nil {
			out.SessionOK = true
		}
	}
	return out, nil
}
