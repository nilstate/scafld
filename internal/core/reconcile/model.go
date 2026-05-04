package reconcile

import (
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

var PhaseBlockFields = map[string]string{
	"status":     "projected phase state",
	"reason":     "human-readable source reason",
	"updated_at": "source event timestamp",
	"source_id":  "session entry identifier",
}

type Projection struct {
	TaskID string
	Lines  []string
}

func Idempotent(current Projection) Projection {
	return Projection{
		TaskID: current.TaskID,
		Lines:  append([]string(nil), current.Lines...),
	}
}

func FromSession(model spec.Model, ledger session.Session) spec.Model {
	replayed := session.Replay(ledger)
	next := model
	for i, criterion := range next.Acceptance.Criteria {
		if state, ok := replayed.CriterionStates[criterion.ID]; ok {
			next.Acceptance.Criteria[i].Status = state.Status
			next.Acceptance.Criteria[i].Evidence = state.Reason
			next.Acceptance.Criteria[i].SourceEvent = state.SourceID
		}
	}
	for pi, phase := range next.Phases {
		if state, ok := replayed.PhaseBlocks[phase.ID]; ok {
			next.Phases[pi].Status = state.Status
			next.Phases[pi].Reason = state.Reason
		}
		for ci, criterion := range phase.Acceptance {
			if state, ok := replayed.CriterionStates[criterion.ID]; ok {
				next.Phases[pi].Acceptance[ci].Status = state.Status
				next.Phases[pi].Acceptance[ci].Evidence = state.Reason
				next.Phases[pi].Acceptance[ci].SourceEvent = state.SourceID
			}
		}
	}
	if len(replayed.Entries) > 0 {
		last := replayed.Entries[len(replayed.Entries)-1]
		next.CurrentState.LatestRunnerUpdate = last.RecordedAt
		next.CurrentState.Reason = last.Reason
	}
	return next
}
