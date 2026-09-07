package taskstate

import (
	"context"
	"errors"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecs struct {
	records []spec.Record
	err     error
}

func (f fakeSpecs) List(context.Context) ([]spec.Record, error) { return f.records, f.err }

type fakeSessions struct {
	ledgers map[string]session.Session
}

func (f fakeSessions) Load(_ context.Context, taskID string) (session.Session, error) {
	ledger, ok := f.ledgers[taskID]
	if !ok {
		return session.Session{}, errors.New("missing session")
	}
	return ledger, nil
}

func TestCurrentExcludesTerminalSpecs(t *testing.T) {
	t.Parallel()

	got, err := Current(context.Background(), fakeSpecs{records: []spec.Record{
		{TaskID: "z-active", Status: spec.StatusActive},
		{TaskID: "a-done", Status: spec.StatusCompleted},
		{TaskID: "a-draft", Status: spec.StatusDraft},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TaskID != "a-draft" || got[1].TaskID != "z-active" {
		t.Fatalf("current = %+v, want stable non-terminal records", got)
	}
}

func TestReadyForFinalizeRequiresPassingReviewEvidence(t *testing.T) {
	t.Parallel()

	passing := session.New("ready", "now").
		WithEntry(session.Entry{Type: "review_override", Status: "accepted", Provider: "human"}).
		WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "human"})
	failing := session.New("blocked", "now").WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "codex"})
	got, err := ReadyForFinalize(context.Background(), fakeSpecs{records: []spec.Record{
		{TaskID: "blocked", Status: spec.StatusReview},
		{TaskID: "ready", Status: spec.StatusReview},
		{TaskID: "draft", Status: spec.StatusDraft},
	}}, fakeSessions{ledgers: map[string]session.Session{"ready": passing, "blocked": failing}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TaskID != "ready" {
		t.Fatalf("ready = %+v, want only accepted review", got)
	}
}
