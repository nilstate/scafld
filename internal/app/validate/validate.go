package validate

import (
	"context"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec loading port used by validation.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// Output reports spec validation status.
type Output struct {
	TaskID string   `json:"task_id"`
	Path   string   `json:"path"`
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

// Run validates a spec without mutating workspace state.
func Run(ctx context.Context, store SpecStore, taskID string) (Output, error) {
	model, path, err := store.Load(ctx, taskID)
	if err != nil {
		return Output{}, fmt.Errorf("load spec: %w", err)
	}
	validation := spec.Validate(model)
	return Output{TaskID: model.TaskID, Path: path, Valid: validation.Valid, Errors: validation.Errors}, nil
}
