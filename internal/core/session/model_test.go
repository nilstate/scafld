package session

import "testing"

func TestReplayProjectionIdempotentAndAppendOrder(t *testing.T) {
	t.Parallel()

	ledger := New("task", "t0")
	ledger = ledger.WithEntry(Entry{ID: "one", Type: "criterion", RecordedAt: "t1", CriterionID: "ac1", Status: "fail"})
	ledger = ledger.WithEntry(Entry{ID: "two", Type: "criterion", RecordedAt: "t2", CriterionID: "ac1", Status: "pass"})
	replayed := Replay(ledger)
	if len(replayed.Entries) != 2 {
		t.Fatalf("entry count = %d", len(replayed.Entries))
	}
	if replayed.CriterionStates["ac1"].Status != "pass" {
		t.Fatalf("state = %+v", replayed.CriterionStates["ac1"])
	}
}
