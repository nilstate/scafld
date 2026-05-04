package handoff

import (
	"context"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec persistence port used by handoff rendering.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// Run renders the model-facing handoff for taskID.
func Run(ctx context.Context, specs SpecStore, taskID string) (string, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("# Handoff: %s\n\nStatus: %s\nNext: %s\n", model.Title, model.Status, model.CurrentState.AllowedFollowUp), nil
}
