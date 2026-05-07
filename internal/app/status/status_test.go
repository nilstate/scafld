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

func TestStatusShowsRunningReviewAttemptAndLatestAcceptedReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	findings := []corereview.Finding{{ID: "old", Severity: corereview.SeverityBlocking, Summary: "old blocker"}}
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeFindings(findings)})
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "running", Reason: "review provider running"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Review.Running || out.Review.AttemptStatus != "running" {
		t.Fatalf("review attempt info missing: %+v", out.Review)
	}
	if out.Review.Attempt == nil || !out.Review.Attempt.Running || out.Review.Attempt.Status != "running" {
		t.Fatalf("nested review attempt info missing: %+v", out.Review)
	}
	if out.Review.Verdict != corereview.VerdictFail || len(out.Review.Findings) != 1 {
		t.Fatalf("latest accepted review should remain visible: %+v", out.Review)
	}
}

func TestStatusIncludesBlockedRepairContract(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Status: spec.StatusBlocked,
		Title:  "Task",
		CurrentState: spec.CurrentState{
			Reason:          "acceptance criteria failed",
			AllowedFollowUp: "scafld handoff task",
		},
		Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{
			ID:          "v1",
			Title:       "tests pass",
			Status:      "fail",
			Evidence:    "exit code was 1",
			SourceEvent: "entry-1",
		}}},
	}
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: session.New("task", "now")}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Gate != "build" || out.TrustedState == "" || out.AllowedFollowUp != "scafld handoff task" {
		t.Fatalf("status repair surface missing: %+v", out)
	}
	if out.Repair == nil || out.Repair.Expected != "all acceptance criteria pass" || len(out.Repair.Blockers) != 1 || len(out.Repair.Evidence) != 1 {
		t.Fatalf("repair contract = %+v", out.Repair)
	}
}
