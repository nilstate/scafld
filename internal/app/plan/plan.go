package plan

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrMissingSpecStore is returned when planning has no spec store.
var ErrMissingSpecStore = errors.New("missing spec store")

// SpecStore is the spec creation port used by planning.
type SpecStore interface {
	CreateDraft(context.Context, spec.Model) (string, error)
}

// Clock supplies planning timestamps.
type Clock interface {
	Now() time.Time
}

// Input describes the operator-provided draft spec fields.
type Input struct {
	TaskID  string
	Title   string
	Summary string
	Command string
	Size    string
	Risk    string
}

// Output describes the created draft spec.
type Output struct {
	TaskID string      `json:"task_id"`
	Path   string      `json:"path"`
	Status spec.Status `json:"status"`
}

// Run creates a draft spec from input.
func Run(ctx context.Context, store SpecStore, clock Clock, input Input) (Output, error) {
	if store == nil {
		return Output{}, ErrMissingSpecStore
	}
	if clock == nil {
		clock = systemClock{}
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	model := spec.Model{
		Version:      "2.0",
		TaskID:       input.TaskID,
		Created:      now,
		Updated:      now,
		Title:        fallback(input.Title, input.TaskID),
		Summary:      fallback(input.Summary, "Implement "+input.TaskID+"."),
		Status:       spec.StatusDraft,
		HardenStatus: spec.HardenNotRun,
		Size:         spec.Size(fallback(input.Size, string(spec.SizeMedium))),
		RiskLevel:    spec.RiskLevel(fallback(input.Risk, string(spec.RiskMedium))),
		CurrentState: spec.CurrentState{
			Next:            "approve",
			Reason:          "draft created",
			Blockers:        "none",
			AllowedFollowUp: "scafld approve " + input.TaskID,
			ReviewGate:      "not_started",
		},
		Acceptance: spec.Acceptance{
			ValidationProfile: "standard",
		},
		Phases: []spec.Phase{{
			ID:        "phase1",
			Number:    1,
			Name:      fallback(input.Title, "Draft "+input.TaskID),
			Status:    "pending",
			Objective: fallback(input.Summary, "Replace this draft objective with the concrete behavior before approval."),
			Changes:   []string{fallback(input.Summary, "Replace this draft change list with concrete files, packages, and behavior before approval.")},
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				Type:         "command",
				Title:        "Configured validation command",
				PhaseID:      "phase1",
				Command:      fallback(input.Command, "go version"),
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pending",
			}},
		}},
		Metadata: map[string]string{"created_by": "scafld"},
		Origin:   spec.Origin{CreatedBy: "scafld", Source: "plan"},
	}
	validation := spec.Validate(model)
	if !validation.Valid {
		return Output{}, validation
	}
	path, err := store.CreateDraft(ctx, model)
	if err != nil {
		return Output{}, fmt.Errorf("create draft spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Path: path, Status: model.Status}, nil
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now().UTC() }

func fallback(value, fb string) string {
	if value == "" {
		return fb
	}
	return value
}
