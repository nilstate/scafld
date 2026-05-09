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

	dossier := corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeDiscover,
		Summary: "Review found an open blocker.",
		Findings: []corereview.Finding{{
			ID:               "f1",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Location:         &corereview.Location{Path: "file.go"},
			Evidence:         "bug",
			Impact:           "test impact",
			Validation:       "rerun test",
			Summary:          "bug",
		}},
		AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: "finding"}},
		Budget:    corereview.Budget{ActualFindings: 1, ActualAttackAngles: 1},
	}
	model := spec.Model{TaskID: "task"}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(dossier)})
	projected := FromSession(model, ledger)
	if projected.Review.Verdict != corereview.VerdictFail || len(projected.Review.Findings) != 1 || projected.Review.Findings[0].Summary != "bug" {
		t.Fatalf("review should project from session: %+v", projected.Review)
	}
}

func TestFromSessionProjectsLifecycleState(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{Type: "approval", Status: "approved"})
	ledger = ledger.WithEntry(session.Entry{Type: "build", Status: string(spec.StatusReview)})
	ledger = ledger.WithEntry(session.Entry{Type: "complete", Status: "completed"})
	projected := FromSession(spec.Model{TaskID: "task", Status: spec.StatusDraft}, ledger)
	if projected.Status != spec.StatusCompleted {
		t.Fatalf("status = %s, want completed", projected.Status)
	}

	buildProjected := FromSession(spec.Model{TaskID: "task", Status: spec.StatusApproved}, session.New("task", "now").WithEntry(session.Entry{Type: "build", Status: string(spec.StatusBlocked)}))
	if buildProjected.Status != spec.StatusBlocked {
		t.Fatalf("build status = %s, want blocked", buildProjected.Status)
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
