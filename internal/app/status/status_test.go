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

func reviewDossier(id string, summary string) corereview.Dossier {
	return corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeDiscover,
		Summary: "Review found an open blocker.",
		Findings: []corereview.Finding{{
			ID:               id,
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Location:         &corereview.Location{Path: "file.go"},
			Evidence:         summary,
			Impact:           "test impact",
			Validation:       "rerun test",
			Summary:          summary,
		}},
		AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: "finding"}},
		Budget:    corereview.Budget{ActualFindings: 1, ActualAttackAngles: 1},
	}
}

func passingReviewDossier(provider string) corereview.Dossier {
	return corereview.Dossier{
		Verdict:  corereview.VerdictPass,
		Mode:     corereview.ModeVerify,
		Provider: provider,
		Summary:  "review passed",
		AttackLog: []corereview.AttackLogEntry{{
			Target: "diff",
			Attack: "scan",
			Result: corereview.AttackResultClean,
		}},
	}
}

func TestStatusIncludesLatestReviewFindings(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier("f1", "bug"))})
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
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier("old", "old blocker"))})
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

func TestStatusDoesNotSurfaceReviewAfterLaterBuildEvidence(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier("old", "old blocker"))})
	ledger = ledger.WithEntry(session.Entry{Type: "build", Status: "active", Reason: "repair started"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{TaskID: "task", Status: spec.StatusActive}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Review.Verdict != "" || len(out.Review.Findings) != 0 {
		t.Fatalf("later build evidence should invalidate stale review info: %+v", out.Review)
	}
}

func TestStatusReviewAttemptFailureCreatesRepairContract(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Output: corereview.EncodeDossier(corereview.Dossier{Verdict: corereview.VerdictPass, Mode: corereview.ModeDiscover, Summary: "clean", AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: "clean"}}})})
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "failed", Reason: "review provider failed: invalid dossier", Path: "/tmp/review-diagnostic.txt"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			AllowedFollowUp: "scafld complete task",
		},
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Repair == nil || out.Repair.Gate != "review" || out.Repair.Next != "scafld handoff task" {
		t.Fatalf("repair contract = %+v", out.Repair)
	}
	if len(out.Repair.Evidence) != 1 || out.Repair.Evidence[0] != "/tmp/review-diagnostic.txt" {
		t.Fatalf("repair evidence = %+v", out.Repair.Evidence)
	}
	if out.Next != "scafld handoff task" {
		t.Fatalf("next = %q, want handoff", out.Next)
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

func TestStatusBlockedRepairContractUsesCurrentPhaseOnly(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task",
		Status: spec.StatusBlocked,
		Title:  "Task",
		CurrentState: spec.CurrentState{
			CurrentPhase:    "phase1",
			Reason:          "phase acceptance failed",
			AllowedFollowUp: "scafld handoff task",
		},
		Phases: []spec.Phase{
			{
				ID: "phase1",
				Acceptance: []spec.Criterion{{
					ID:          "p1",
					Title:       "current phase test",
					Status:      "fail",
					Evidence:    "exit code was 1",
					SourceEvent: "entry-1",
				}},
			},
			{
				ID: "phase2",
				Acceptance: []spec.Criterion{{
					ID:     "p2",
					Title:  "future phase test",
					Status: "pending",
				}},
			},
		},
		Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{
			ID:     "final",
			Title:  "final acceptance",
			Status: "pending",
		}}},
	}
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: session.New("task", "now")}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Repair == nil {
		t.Fatal("repair contract missing")
	}
	if len(out.Repair.Blockers) != 1 || out.Repair.Blockers[0] != "p1: current phase test (exit code was 1)" {
		t.Fatalf("repair blockers = %+v", out.Repair.Blockers)
	}
}

func TestStatusCompletedShowsTerminalCompletionAuthority(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{ID: "review-old", Type: "review", Status: corereview.VerdictFail, Provider: "codex", Output: corereview.EncodeDossier(reviewDossier("old", "old blocker"))})
	ledger = ledger.WithEntry(session.Entry{ID: "review-pass", Type: "review", Status: corereview.VerdictPass, Provider: "codex", Output: corereview.EncodeDossier(passingReviewDossier("codex"))})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusCompleted,
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Completion == nil || out.Completion.Status != "valid" || out.Completion.Kind != "review" || out.Completion.Provider != "codex" || out.Completion.ReviewEvent != "review-pass" {
		t.Fatalf("completion authority = %+v", out.Completion)
	}
	if out.Review.Verdict != corereview.VerdictPass || len(out.Review.Findings) != 0 {
		t.Fatalf("latest review should be the terminal pass, not old failure: %+v", out.Review)
	}
}

func TestStatusCompletedFlagsMissingCompletionAuthority(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{ID: "review-fail", Type: "review", Status: corereview.VerdictFail, Provider: "codex", Output: corereview.EncodeDossier(reviewDossier("old", "old blocker"))})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusCompleted,
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Completion == nil || out.Completion.Status != "invalid" || out.Completion.Reason != "latest review gate has not passed" || out.Completion.Actual != "latest review verdict fail" {
		t.Fatalf("completion authority = %+v", out.Completion)
	}
}
