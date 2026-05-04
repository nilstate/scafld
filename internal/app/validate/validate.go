package validate

import (
	"context"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

type Output struct {
	TaskID string
	Path   string
	Valid  bool
	Errors []string
}

func Run(ctx context.Context, store SpecStore, taskID string) (Output, error) {
	model, path, err := store.Load(ctx, taskID)
	if err != nil {
		return Output{}, fmt.Errorf("load spec: %w", err)
	}
	validation := spec.Validate(model)
	return Output{TaskID: model.TaskID, Path: path, Valid: validation.Valid, Errors: validation.Errors}, nil
}
