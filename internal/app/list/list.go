package list

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	List(context.Context) ([]spec.Record, error)
}

func Run(ctx context.Context, store SpecStore) ([]spec.Record, error) {
	return store.List(ctx)
}
