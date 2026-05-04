package report

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type SpecStore interface {
	List(context.Context) ([]spec.Record, error)
}

type Output struct {
	Total    int
	ByStatus map[spec.Status]int
}

func Run(ctx context.Context, store SpecStore) (Output, error) {
	records, err := store.List(ctx)
	if err != nil {
		return Output{}, err
	}
	out := Output{Total: len(records), ByStatus: map[spec.Status]int{}}
	for _, record := range records {
		out.ByStatus[record.Status]++
	}
	return out, nil
}
