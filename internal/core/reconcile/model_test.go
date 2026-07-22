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

	model := spec.Model{TaskID: "task", Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "pending"}}}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{ID: "e1", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Reason: "evidence", Command: "true", ExpectedKind: string(acceptance.ExpectedExitCodeZero), CriterionType: "command"})
	projected := FromSession(model, ledger)
	if projected.Phases[0].Acceptance[0].Status != "pass" {
		t.Fatalf("criterion should project from session: %+v", projected.Phases[0].Acceptance[0])
	}
}

func TestFromSessionRejectsCriterionEvidenceMissingContractFields(t *testing.T) {
	t.Parallel()

	base := session.Entry{
		ID:            "entry-old",
		Type:          "criterion",
		CriterionID:   "ac1",
		PhaseID:       "phase1",
		Status:        "pass",
		Reason:        "exit code was 0",
		Command:       "go test ./phase",
		ExpectedKind:  string(acceptance.ExpectedExitCodeZero),
		CriterionType: "command",
	}
	for name, mutate := range map[string]func(*session.Entry){
		"missing expected kind":  func(entry *session.Entry) { entry.ExpectedKind = "" },
		"missing criterion type": func(entry *session.Entry) { entry.CriterionType = "" },
		"missing phase id":       func(entry *session.Entry) { entry.PhaseID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry := base
			mutate(&entry)
			model := spec.Model{TaskID: "task", Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{
				ID:           "ac1",
				PhaseID:      "phase1",
				Command:      "go test ./phase",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pass",
				Evidence:     "exit code was 0",
				SourceEvent:  "entry-old",
			}}}}}
			projected := FromSession(model, session.New("task", "now").WithEntry(entry))
			got := projected.Phases[0].Acceptance[0]
			if got.Status != "pending" || got.Evidence != "" || got.SourceEvent != "" {
				t.Fatalf("incomplete evidence projected: %+v", got)
			}
		})
	}
}

func TestFromSessionInvalidatesCriterionEvidenceAfterCommandChange(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			CurrentPhase:    "final",
			Next:            "review",
			AllowedFollowUp: "scafld review task",
		},
		Phases: []spec.Phase{{
			ID:     "phase1",
			Name:   "Phase",
			Status: "completed",
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				PhaseID:      "phase1",
				Command:      "new check",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pass",
				Evidence:     "exit code was 0",
				SourceEvent:  "entry-old",
			}},
		}},
	}
	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "entry-old", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Reason: "exit code was 0", Command: "old check", ExpectedKind: string(acceptance.ExpectedExitCodeZero), CriterionType: "command"})
	ledger = ledger.WithEntry(session.Entry{ID: "entry-phase", Type: "phase", PhaseID: "phase1", Status: "completed", Reason: "all phase criteria passed"})
	ledger = ledger.WithEntry(session.Entry{ID: "entry-build", Type: "build", Status: string(spec.StatusReview), Reason: "build completed; ready for review"})

	projected := FromSession(model, ledger)
	criterion := projected.Phases[0].Acceptance[0]
	if criterion.Status != "pending" || criterion.Evidence != "" || criterion.SourceEvent != "" {
		t.Fatalf("stale criterion evidence projected: %+v", criterion)
	}
	if projected.Phases[0].Status != "active" || projected.Phases[0].Reason != acceptanceStaleReason {
		t.Fatalf("phase should reopen after stale evidence: %+v", projected.Phases[0])
	}
	if projected.Status != spec.StatusActive || projected.CurrentState.CurrentPhase != "phase1" || projected.CurrentState.Next != "build" {
		t.Fatalf("model should return to build after stale evidence: status=%s current=%+v", projected.Status, projected.CurrentState)
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
