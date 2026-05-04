package approve

import (
	"context"
	"fmt"
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

type Output struct {
	TaskID string
	Status spec.Status
	Path   string
}

func Run(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, taskID string) (Output, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	model.Status = spec.StatusApproved
	model.CurrentState.Next = "build"
	model.CurrentState.AllowedFollowUp = "scafld build " + model.TaskID
	now := clock.Now().UTC().Format(time.RFC3339)
	model.Updated = now
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save approved spec: %w", err)
	}
	if sessions != nil {
		_, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "approval", Status: "approved", Reason: "spec approved"}, now)
		if err != nil {
			return Output{}, fmt.Errorf("append approval: %w", err)
		}
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Path: path}, nil
}
