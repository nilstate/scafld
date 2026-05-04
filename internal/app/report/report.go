package report

import (
	"context"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec listing port used by reports.
type SpecStore interface {
	List(context.Context) ([]spec.Record, error)
}

// Output summarizes workspace task counts.
type Output struct {
	Total    int                 `json:"total"`
	ByStatus map[spec.Status]int `json:"by_status"`
}

// Run aggregates spec records by status.
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
