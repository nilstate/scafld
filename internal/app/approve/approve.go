package approve

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrSpecNotDraft is returned when approval is attempted outside draft state.
var ErrSpecNotDraft = errors.New("approve only operates on drafts")

// SpecStore is the spec persistence port used by approval.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
	Find(string) (string, error)
}

// SessionStore is the session evidence port used by approval.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
}

// WorkspaceStatus captures dirty workspace state before task execution begins.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies approval timestamps.
type Clock interface{ Now() time.Time }

// Output describes the approved task.
type Output struct {
	TaskID string      `json:"task_id"`
	Status spec.Status `json:"status"`
	Path   string      `json:"path"`
}

// Run approves a draft spec and records approval evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, clock Clock, taskID string) (Output, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	if model.Status != spec.StatusDraft {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotDraft, model.Status)
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if sessions != nil {
		if workspace != nil {
			snapshot, err := workspace.ChangedFiles(ctx)
			if err != nil {
				return Output{}, fmt.Errorf("capture workspace baseline: %w", err)
			}
			_, err = sessions.Append(ctx, model.TaskID, session.Entry{
				Type:   session.EntryWorkspaceBaseline,
				Status: "captured",
				Reason: fmt.Sprintf("workspace baseline captured before approval: %d changed path(s)", len(snapshot)),
				Output: strings.Join(snapshot, "\n"),
			}, now)
			if err != nil {
				return Output{}, fmt.Errorf("append workspace baseline: %w", err)
			}
		}
		_, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "approval", Status: "approved", Reason: "spec approved"}, now)
		if err != nil {
			return Output{}, fmt.Errorf("append approval: %w", err)
		}
	}
	model.Status = spec.StatusApproved
	model.CurrentState.Next = "build"
	model.CurrentState.AllowedFollowUp = "scafld build " + model.TaskID
	model.Updated = now
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save approved spec: %w", err)
	}
	path, err = specs.Find(model.TaskID)
	if err != nil {
		return Output{}, fmt.Errorf("find approved spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Path: path}, nil
}
