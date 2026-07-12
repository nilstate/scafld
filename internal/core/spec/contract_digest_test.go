package spec

import "testing"

// TestContractDigestStableAcrossPlanningLogTimestamp proves that
// ContractDigest is stable for byte-identical task contracts that differ only
// in a planning-log timestamp. The planning log is a volatile projection field
// (it carries per-event timestamps), so it must not participate in the
// contract digest, or a valid review is falsely flagged stale.
//
// This is a regression test for the defect where PlanningLog was included in
// the digest struct, causing two otherwise-identical contracts to hash
// differently and triggering spurious review_stale_after_spec re-reviews.
func TestContractDigestStableAcrossPlanningLogTimestamp(t *testing.T) {
	t.Parallel()

	base := func() Model {
		return Model{
			Version: "2.0",
			TaskID:  "demo-task",
			Title:   "demo title",
			Context: Context{Packages: []string{"internal/core/spec"}},
		}
	}

	a := base()
	a.PlanningLog = []PlanningEvent{{Time: "2026-01-01T00:00:00Z", Text: "planned"}}

	b := base()
	b.PlanningLog = []PlanningEvent{{Time: "2026-06-01T00:00:00Z", Text: "planned"}}

	if got, want := ContractDigest(a), ContractDigest(b); got != want {
		t.Fatalf("digest not stable across planning-log timestamp: %s != %s", got, want)
	}
}

// TestContractDigestChangesWhenContractChanges proves the digest does change
// for a real, semantically meaningful contract edit (objective text), so the
// staleness signal is still live for genuine changes.
func TestContractDigestChangesWhenContractChanges(t *testing.T) {
	t.Parallel()

	a := Model{
		Version:     "2.0",
		TaskID:      "demo-task",
		Title:       "demo title",
		Objectives:  []string{"do the thing"},
		PlanningLog: []PlanningEvent{{Time: "2026-01-01T00:00:00Z", Text: "planned"}},
	}
	b := a
	b.Objectives = []string{"do the OTHER thing"}

	if got, want := ContractDigest(a), ContractDigest(b); got == want {
		t.Fatalf("digest should change when contract objective changes: both = %s", got)
	}
}
