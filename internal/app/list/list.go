package list

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec listing port.
type SpecStore interface {
	List(context.Context) ([]spec.Record, error)
}

// Run lists current task specs.
func Run(ctx context.Context, store SpecStore) ([]spec.Record, error) {
	return store.List(ctx)
}
