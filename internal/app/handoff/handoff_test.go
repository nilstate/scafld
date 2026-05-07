package handoff

import (
	"context"
	"strings"
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestGoldenHandoffRepairRecoveryChallengerExecutor(t *testing.T) {
	t.Parallel()
}

type fakeSpecStore struct{ model spec.Model }

func (f fakeSpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}

type fakeSessionStore struct{ ledger session.Session }

func (f fakeSessionStore) Load(context.Context, string) (session.Session, error) {
	return f.ledger, nil
}

func TestHandoffIncludesLatestReviewFindings(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	findings := []corereview.Finding{{ID: "f1", Severity: corereview.SeverityBlocking, Summary: "bug"}}
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeFindings(findings)})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Review Findings") || !strings.Contains(out, "bug") {
		t.Fatalf("handoff missing review findings:\n%s", out)
	}
}

func TestHandoffIncludesBlockedAcceptanceEvidence(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusBlocked,
		Phases: []spec.Phase{{
			ID:   "phase1",
			Name: "Implementation",
			Acceptance: []spec.Criterion{{
				ID:      "ac1",
				Title:   "API tests",
				PhaseID: "phase1",
				Command: "go test ./api",
			}},
		}},
	}
	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "fail", Reason: "exit code 1", Command: "go test ./api"})
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Blocked Acceptance", "[fail] API tests", "exit code 1", "Command: `go test ./api`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("handoff missing %q:\n%s", want, out)
		}
	}
}

func TestHandoffReviewStateIncludesContractEvidenceAndReviewerFocus(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID:      "task",
		Title:       "Task",
		Status:      spec.StatusReview,
		Summary:     "Ship the governed change.",
		Scope:       []string{"Review baseline behavior."},
		Touchpoints: []string{"internal/app/review"},
		CurrentState: spec.CurrentState{
			Reason:          "build completed",
			ReviewGate:      "not_started",
			AllowedFollowUp: "scafld review task",
		},
		Phases: []spec.Phase{{
			ID:   "phase1",
			Name: "Implementation",
			Acceptance: []spec.Criterion{{
				ID:      "ac1",
				Title:   "Full check",
				PhaseID: "phase1",
				Command: "make check",
			}},
		}},
	}
	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{ID: "entry-1", Type: session.EntryWorkspaceBaseline, Status: "captured", Output: "README.md\n"})
	ledger = ledger.WithEntry(session.Entry{ID: "entry-2", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Reason: "exit code was 0", Command: "make check"})
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## Task Contract",
		"Ship the governed change.",
		"Review baseline behavior.",
		"## Acceptance Evidence",
		"[pass] Full check",
		"Source event: `entry-2`",
		"## Review Gate",
		"Workspace baseline: `entry-1`",
		"Reviewer focus:",
		"Treat session evidence as trusted state",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("handoff missing %q:\n%s", want, out)
		}
	}
}
