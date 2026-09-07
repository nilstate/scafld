// Package taskstate contains the small workspace projections shared by
// taskless lifecycle entry points.
package taskstate

import (
	"context"
	"sort"

	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecLister is the read-only workspace listing needed to resolve taskless
// lifecycle calls without guessing which task an agent owns.
type SpecLister interface {
	List(context.Context) ([]spec.Record, error)
}

// SessionLoader is the read-only evidence lookup used to identify tasks whose
// accepted review is actually ready for deterministic finalization.
type SessionLoader interface {
	Load(context.Context, string) (session.Session, error)
}

// Current returns non-terminal task specs in stable task-id order.
func Current(ctx context.Context, specs SpecLister) ([]spec.Record, error) {
	records, err := specs.List(ctx)
	if err != nil {
		return nil, err
	}
	current := make([]spec.Record, 0, len(records))
	for _, record := range records {
		if record.TaskID == "" || isTerminal(record.Status) {
			continue
		}
		current = append(current, record)
	}
	sort.Slice(current, func(i, j int) bool { return current[i].TaskID < current[j].TaskID })
	return current, nil
}

// ReadyForFinalize returns current review tasks with a valid accepted review.
// A task is not selected merely because it is in the review phase: pending or
// failed review work must remain visible to an explicitly addressed command.
func ReadyForFinalize(ctx context.Context, specs SpecLister, sessions SessionLoader) ([]spec.Record, error) {
	current, err := Current(ctx, specs)
	if err != nil {
		return nil, err
	}
	ready := make([]spec.Record, 0, len(current))
	for _, record := range current {
		if record.Status != spec.StatusReview || sessions == nil {
			continue
		}
		ledger, err := sessions.Load(ctx, record.TaskID)
		if err != nil {
			continue
		}
		if reviewgate.CurrentReviewGate(ledger).Valid {
			ready = append(ready, record)
		}
	}
	return ready, nil
}

func isTerminal(status spec.Status) bool {
	switch status {
	case spec.StatusCompleted, spec.StatusFailed, spec.StatusCancelled:
		return true
	default:
		return false
	}
}
