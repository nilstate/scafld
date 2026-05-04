package status

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec loading port used by status.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore is the session loading port used by status.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// Output describes the task status projection.
type Output struct {
	TaskID    string      `json:"task_id"`
	Status    spec.Status `json:"status"`
	Title     string      `json:"title"`
	Next      string      `json:"next"`
	SessionOK bool        `json:"session_ok"`
}

// Run reads status for taskID.
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
