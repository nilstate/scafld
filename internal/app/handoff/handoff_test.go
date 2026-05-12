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

func reviewDossier() corereview.Dossier {
	return corereview.Dossier{
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
}

func TestHandoffIncludesLatestReviewFindings(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier())})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Review Dossier") || !strings.Contains(out, "bug") {
		t.Fatalf("handoff missing review findings:\n%s", out)
	}
}

func TestHandoffDoesNotSurfaceReviewAfterLaterBuildEvidence(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier())})
	ledger = ledger.WithEntry(session.Entry{Type: "build", Status: "active", Reason: "repair started"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusActive}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "## Review Dossier") || strings.Contains(out, "bug") {
		t.Fatalf("handoff surfaced stale review findings:\n%s", out)
	}
}

func TestHandoffKeepsAcceptedReviewDuringLaterAttempt(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier())})
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "running", Reason: "review provider running"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "## Review Dossier") || !strings.Contains(out, "bug") {
		t.Fatalf("handoff should keep latest accepted review during later attempt:\n%s", out)
	}
}

func TestHandoffUsesRepairNextAfterFailedReviewAttempt(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Output: corereview.EncodeDossier(corereview.Dossier{Verdict: corereview.VerdictPass, Mode: corereview.ModeDiscover, Summary: "clean", AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: "clean"}}})})
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "failed", Reason: "review provider failed", Path: "/tmp/scafld-review.txt"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			AllowedFollowUp: "scafld complete task",
		},
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Next: scafld handoff task", "Allowed command: `scafld handoff task`", "Latest review attempt: failed", "Attempt reason: review provider failed", "Attempt diagnostic: `/tmp/scafld-review.txt`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("handoff missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Allowed command: `scafld complete task`") {
		t.Fatalf("handoff points back to complete:\n%s", out)
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

func TestHandoffBlockedAcceptanceUsesCurrentPhaseOnly(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusBlocked,
		CurrentState: spec.CurrentState{
			CurrentPhase: "phase1",
		},
		Phases: []spec.Phase{
			{
				ID:   "phase1",
				Name: "Current",
				Acceptance: []spec.Criterion{{
					ID:      "ac1",
					Title:   "Current tests",
					PhaseID: "phase1",
					Command: "go test ./current",
				}},
			},
			{
				ID:   "phase2",
				Name: "Future",
				Acceptance: []spec.Criterion{{
					ID:      "ac2",
					Title:   "Future tests",
					PhaseID: "phase2",
					Command: "go test ./future",
				}},
			},
		},
		Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{
			ID:      "final",
			Title:   "Final tests",
			Command: "go test ./...",
		}}},
	}
	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "fail", Reason: "exit code 1", Command: "go test ./current"})
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	blockedSection := out
	if idx := strings.Index(out, "## Blocked Acceptance"); idx >= 0 {
		blockedSection = out[idx:]
	}
	if !strings.Contains(blockedSection, "Current tests") || strings.Contains(blockedSection, "Future tests") || strings.Contains(blockedSection, "Final tests") {
		t.Fatalf("blocked acceptance should only surface current phase blockers:\n%s", out)
	}
}

func TestHandoffIncludesActiveBuildStep(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusActive,
		CurrentState: spec.CurrentState{
			CurrentPhase:    "phase1",
			AllowedFollowUp: "scafld handoff task",
		},
		Phases: []spec.Phase{{
			ID:        "phase1",
			Name:      "Implementation",
			Objective: "Wire the lifecycle step.",
			Changes:   []string{"internal/app/build/build.go"},
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				Title:        "Build tests",
				PhaseID:      "phase1",
				Command:      "go test ./internal/app/build",
				ExpectedKind: "exit_code_zero",
			}},
		}},
	}
	out, err := Run(context.Background(), fakeSpecStore{model: model}, nil, "task")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Build Step", "Current phase: phase1", "Wire the lifecycle step.", "internal/app/build/build.go", "After implementing this phase, run `scafld build task`"} {
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
