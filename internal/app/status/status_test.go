package status

import (
	"context"
	"testing"
	"time"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/testkit/sessiontest"
)

type fakeSpecStore struct{ model spec.Model }

func (f fakeSpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}

type fakeSessionStore struct{ ledger session.Session }

func (f fakeSessionStore) Load(context.Context, string) (session.Session, error) {
	return f.ledger, nil
}

type fakeWorkspace struct {
	snapshot []string
	head     string
	material reviewevidence.MaterialSeal
}

func (f fakeWorkspace) ChangedFiles(context.Context) ([]string, error) {
	return append([]string(nil), f.snapshot...), nil
}

func (f fakeWorkspace) ResolveHead(context.Context) (string, bool, error) {
	if f.head == "" {
		return "", false, nil
	}
	return f.head, true, nil
}

func (f fakeWorkspace) MaterialSeal(context.Context, []string) (reviewevidence.MaterialSeal, error) {
	return f.material, nil
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

func TestStatusIncludesTaskMaterialProjection(t *testing.T) {
	t.Parallel()

	reviewEntry := sessiontest.PassingReviewEntry("review-pass", "codex")
	reviewEntry.ReviewedScope = []string{"api"}
	reviewEntry.ReviewedMaterialDigest = "same-material"
	ledger := session.New("task", "2026-05-05T00:00:00Z").
		WithEntry(session.Entry{ID: "baseline", Type: session.EntryWorkspaceBaseline, Status: "captured", Output: " M old api/handler.go\n M old docs/index.md\n"}).
		WithEntry(reviewEntry)
	model := spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		Context: spec.Context{
			FilesImpacted: []string{"`api/handler.go`"},
		},
	}
	out, err := Run(context.Background(), fakeSpecStore{model: model}, fakeSessionStore{ledger: ledger}, "task", fakeWorkspace{
		snapshot: []string{" M new api/handler.go", " M new docs/index.md"},
		head:     "head",
		material: reviewevidence.MaterialSeal{Scope: []string{"api"}, Digest: "same-material"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TaskMaterial == nil {
		t.Fatal("task material missing")
	}
	if out.TaskMaterial.MaterialStatus != "unchanged" || out.TaskMaterial.CurrentMaterialDigest != "same-material" {
		t.Fatalf("material status = %+v", out.TaskMaterial)
	}
	if len(out.TaskMaterial.Scope) != 1 || out.TaskMaterial.Scope[0] != "api/handler.go" {
		t.Fatalf("scope = %+v", out.TaskMaterial.Scope)
	}
	if len(out.TaskMaterial.TaskChanges) != 1 || out.TaskMaterial.TaskChanges[0] != "changed api/handler.go (M old -> M new)" {
		t.Fatalf("task changes = %+v", out.TaskMaterial.TaskChanges)
	}
	if len(out.TaskMaterial.AmbientDrift) != 1 || out.TaskMaterial.AmbientDrift[0] != "changed docs/index.md (M old -> M new)" {
		t.Fatalf("ambient drift = %+v", out.TaskMaterial.AmbientDrift)
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
	if out.NextAction.Role != "operator" || out.NextAction.Action != "repair_review_provider" || out.NextAction.ThenCommand != "scafld review task" {
		t.Fatalf("next action = %+v", out.NextAction)
	}
	if len(out.Repair.Evidence) != 1 || out.Repair.Evidence[0] != "/tmp/review-diagnostic.txt" {
		t.Fatalf("repair evidence = %+v", out.Repair.Evidence)
	}
	if out.Next != "scafld handoff task" {
		t.Fatalf("next = %q, want handoff", out.Next)
	}
}

func TestStatusReviewStaleAttemptCreatesRecoveryContract(t *testing.T) {
	t.Parallel()

	recordedAt := time.Now().UTC().Add(-3 * time.Hour).Format(time.RFC3339)
	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review_attempt", Status: "running", RecordedAt: recordedAt, Reason: "provider stopped reporting"})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Repair == nil || out.Repair.Next != "scafld review task" || out.Review.AttemptStatus != "stale" {
		t.Fatalf("stale repair = %+v review=%+v", out.Repair, out.Review)
	}
	if out.NextAction.Action != "recover_stale_review_attempt" || out.NextAction.Command != "scafld review task" {
		t.Fatalf("next action = %+v", out.NextAction)
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
	if out.NextAction.Role != "executor" || out.NextAction.Action != "repair_acceptance" || out.NextAction.AfterCommand != "scafld build task" {
		t.Fatalf("next action = %+v", out.NextAction)
	}
	if out.Repair == nil || out.Repair.Expected != "all acceptance criteria pass" || len(out.Repair.Blockers) != 1 || len(out.Repair.Evidence) != 1 {
		t.Fatalf("repair contract = %+v", out.Repair)
	}
}

func TestStatusReviewFailureNextActionRefreshesBuildBeforeReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "2026-05-05T00:00:00Z")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(reviewDossier("f1", "bug"))})
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			AllowedFollowUp: "scafld handoff task",
		},
	}}, fakeSessionStore{ledger: ledger}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.NextAction.Role != "executor" || out.NextAction.Action != "repair_review_findings" {
		t.Fatalf("next action = %+v", out.NextAction)
	}
	if out.NextAction.Command != "scafld handoff task" || out.NextAction.AfterCommand != "scafld build task" || out.NextAction.ThenCommand != "scafld review task" {
		t.Fatalf("next action commands = %+v", out.NextAction)
	}
}

func TestStatusReviewWithoutLedgerReviewSuggestsReviewNotComplete(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			Reason:          "exit code was 0",
			AllowedFollowUp: "scafld complete task",
		},
	}}, fakeSessionStore{ledger: session.New("task", "now")}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Next != "scafld review task" || out.AllowedFollowUp != "scafld review task" {
		t.Fatalf("next = %q allowed = %q, want review", out.Next, out.AllowedFollowUp)
	}
	if out.NextAction.Action != "run_review" || out.NextAction.Command != "scafld review task" {
		t.Fatalf("next action = %+v", out.NextAction)
	}
	if out.Review.Reason != "latest review gate has not passed" {
		t.Fatalf("review reason = %q", out.Review.Reason)
	}
}

func TestStatusReviewPassRequiresValidLedgerAuthority(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now").
		WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "local"})
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
	if out.NextAction.Action == "complete" || out.NextAction.Command != "scafld review task" {
		t.Fatalf("next action = %+v", out.NextAction)
	}
}

func TestStatusReviewWithValidLedgerReviewSuggestsComplete(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("", "codex"))
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
	if out.Next != "scafld complete task" || out.NextAction.Action != "complete" || out.NextAction.Command != "scafld complete task" {
		t.Fatalf("status = %+v", out)
	}
}

func TestStatusMaterialSealKeepsCompleteActionDespiteAmbientDrift(t *testing.T) {
	t.Parallel()

	material := reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "same-material"}
	entry := sessiontest.PassingReviewEntry("", "codex")
	entry.ReviewedScope = append([]string(nil), material.Scope...)
	entry.ReviewedMaterialDigest = material.Digest
	ledger := session.New("task", "now").WithEntry(entry)
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			AllowedFollowUp: "scafld complete task",
		},
	}}, fakeSessionStore{ledger: ledger}, "task", fakeWorkspace{
		head:     "different-head",
		snapshot: []string{" M adjacent docs/readme.md"},
		material: material,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Next != "scafld complete task" || out.NextAction.Action != "complete" {
		t.Fatalf("status = %+v", out)
	}
}

func TestStatusMaterialSealStaleReviewSuggestsReview(t *testing.T) {
	t.Parallel()

	entry := sessiontest.PassingReviewEntry("", "codex")
	entry.ReviewedScope = []string{"api/handler.go"}
	entry.ReviewedMaterialDigest = "reviewed-material"
	ledger := session.New("task", "now").WithEntry(entry)
	out, err := Run(context.Background(), fakeSpecStore{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusReview,
		CurrentState: spec.CurrentState{
			AllowedFollowUp: "scafld complete task",
		},
	}}, fakeSessionStore{ledger: ledger}, "task", fakeWorkspace{
		head:     "head",
		snapshot: []string{" M changed api/handler.go"},
		material: reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "changed-material"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Next != "scafld review task" || out.Repair == nil || out.Repair.Reason != "latest review is stale against current task material" {
		t.Fatalf("status = %+v", out)
	}
	if out.NextAction.Role != "reviewer" || out.NextAction.Action != "run_review" || out.NextAction.Command != "scafld review task" {
		t.Fatalf("next action = %+v", out.NextAction)
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
	ledger = ledger.WithEntry(session.Entry{ID: "build-repair", Type: "build", Status: string(spec.StatusReview), Reason: "review repair evidence refreshed"})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-pass", "codex"))
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
