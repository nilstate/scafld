package reconcile

import (
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestGoldenProjectionSourceOfTruthCriterionEvidence(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task", Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"}}}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{ID: "e1", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Reason: "evidence"})
	projected := FromSession(model, ledger)
	if projected.Phases[0].Acceptance[0].Status != "pass" {
		t.Fatalf("criterion should project from session: %+v", projected.Phases[0].Acceptance[0])
	}
}

func TestFromSessionProjectsLatestReviewFindings(t *testing.T) {
	t.Parallel()

	findings := []corereview.Finding{{ID: "f1", Severity: corereview.SeverityBlocking, Summary: "bug"}}
	model := spec.Model{TaskID: "task"}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeFindings(findings)})
	projected := FromSession(model, ledger)
	if projected.Review.Verdict != corereview.VerdictFail || len(projected.Review.Findings) != 1 || projected.Review.Findings[0].Summary != "bug" {
		t.Fatalf("review should project from session: %+v", projected.Review)
	}
}

func TestReconcileContentionRaceScenario(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task"}
	ledger := session.New("task", "now")
	done := make(chan spec.Model, 8)
	for i := 0; i < 8; i++ {
		go func() { done <- FromSession(model, ledger) }()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}
