package status

import (
	"context"
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecStore struct{ model spec.Model }

func (f fakeSpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}

type fakeSessionStore struct{ ledger session.Session }

func (f fakeSessionStore) Load(context.Context, string) (session.Session, error) {
	return f.ledger, nil
}

func TestStatusIncludesLatestReviewFindings(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	findings := []corereview.Finding{{ID: "f1", Severity: corereview.SeverityBlocking, Summary: "bug"}}
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeFindings(findings)})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Review.Verdict != corereview.VerdictFail || len(out.Review.Findings) != 1 || out.Review.Findings[0].Summary != "bug" {
		t.Fatalf("review info = %+v", out.Review)
	}
}

func TestStatusShowsRunningReviewAttemptInsteadOfPreviousReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	findings := []corereview.Finding{{ID: "old", Severity: corereview.SeverityBlocking, Summary: "old blocker"}}
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeFindings(findings)})
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "running", Reason: "review provider running"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Review.Running || out.Review.Verdict != "" || len(out.Review.Findings) != 0 {
		t.Fatalf("review info should expose running attempt instead of stale review: %+v", out.Review)
	}
}
