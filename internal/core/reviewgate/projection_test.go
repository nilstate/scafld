package reviewgate

import (
	"testing"
	"time"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/testkit/sessiontest"
)

func TestProjectClassifiesAttemptLeaseStates(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	model := spec.Model{TaskID: "task", Status: spec.StatusReview}

	running := session.New("task", now.Format(time.RFC3339)).
		WithEntry(session.Entry{Type: EntryReviewAttempt, Status: AttemptStatusRunning, RecordedAt: now.Add(-time.Minute).Format(time.RFC3339), LeaseExpiresAt: now.Add(time.Hour).Format(time.RFC3339)})
	if state := Project(running, model, Options{Now: now}); state.Kind != KindAttemptRunning || state.CanStartReview {
		t.Fatalf("running state = %+v", state)
	}

	stale := session.New("task", now.Format(time.RFC3339)).
		WithEntry(session.Entry{Type: EntryReviewAttempt, Status: AttemptStatusRunning, RecordedAt: now.Add(-DefaultAttemptLease - time.Minute).Format(time.RFC3339)})
	state := Project(stale, model, Options{Now: now})
	if state.Kind != KindAttemptStale || !state.CanStartReview || !state.ShouldAbandonAttempt {
		t.Fatalf("stale state = %+v", state)
	}
}

func TestProjectBlocksFailedReviewUntilBuildEvidence(t *testing.T) {
	t.Parallel()

	dossier := corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeDiscover,
		Summary: "Review found an open blocker.",
		Findings: []corereview.Finding{{
			ID:               "f1",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Confidence:       corereview.ConfidenceHigh,
			Location:         &corereview.Location{Path: "file.go", Line: 1},
			Evidence:         "bug evidence",
			Impact:           "bug impact",
			Validation:       "rerun test",
			Summary:          "bug",
		}},
		AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: corereview.AttackResultFinding}},
	}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{ID: "review-fail", Type: "review", Status: corereview.VerdictFail, Output: corereview.EncodeDossier(dossier)})
	model := spec.Model{TaskID: "task", Status: spec.StatusReview}

	state := Project(ledger, model, Options{Now: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)})
	if state.Kind != KindReviewFailed || state.CanStartReview || !state.ReviewBlockedUntilBuild || len(state.Blockers) != 1 {
		t.Fatalf("failed review state = %+v", state)
	}

	ledger = ledger.WithEntry(session.Entry{ID: "build-1", Type: "build", Status: string(spec.StatusReview)})
	state = Project(ledger, model, Options{Now: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)})
	if state.Kind != KindReviewStaleAfterBuild || !state.CanStartReview || !state.HasReview {
		t.Fatalf("post-build state = %+v", state)
	}
}

func TestProjectBlocksReviewRerunWhenBuildAddsNoMaterialRepairEvidence(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task", Status: spec.StatusReview, Summary: "contract"}
	dossier := corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeVerify,
		Summary: "Review found a persistent blocker.",
		Findings: []corereview.Finding{{
			ID:               "f1",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Category:         "correctness",
			Confidence:       corereview.ConfidenceHigh,
			Location:         &corereview.Location{Path: "file.go", Line: 1},
			Evidence:         "bug evidence",
			Impact:           "bug impact",
			Validation:       "go test ./...",
			Summary:          "bug still exists",
		}},
		AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: corereview.AttackResultFinding}},
	}
	failedReview := session.Entry{
		ID:                     "review-fail",
		Type:                   "review",
		Status:                 corereview.VerdictFail,
		Output:                 corereview.EncodeDossier(dossier),
		ReviewedSpec:           spec.ContractDigest(model),
		ReviewedScope:          []string{"file.go"},
		ReviewedMaterialDigest: "same-material",
	}
	ledger := session.New("task", "now").
		WithEntry(failedReview).
		WithEntry(session.Entry{ID: "build-1", Type: "build", Status: string(spec.StatusReview)})

	state := Project(ledger, model, Options{
		Now:             time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		MaterialSeal:    reviewevidence.MaterialSeal{Scope: []string{"file.go"}, Digest: "same-material"},
		HasMaterialSeal: true,
	})
	if state.Kind != KindReviewNeedsOperatorDecision || state.CanStartReview || !state.ReviewRerunBlocked || !state.OperatorDecisionRequired {
		t.Fatalf("unchanged repair state = %+v", state)
	}
	if !state.HasReview || !state.HasPriorFailedReview || len(state.Blockers) != 1 || len(state.BlockerFingerprints) != 1 {
		t.Fatalf("failed review context missing from state: %+v", state)
	}
	if scope := ReviewRerunMaterialScope(ledger); len(scope) != 1 || scope[0] != "file.go" {
		t.Fatalf("rerun material scope = %+v", scope)
	}
}

func TestProjectAllowsReviewRerunWhenMaterialChangedAfterFailedReview(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task", Status: spec.StatusReview, Summary: "contract"}
	failedReview := session.Entry{
		ID:                     "review-fail",
		Type:                   "review",
		Status:                 corereview.VerdictFail,
		Output:                 corereview.EncodeDossier(blockingDossierForProjectionTest()),
		ReviewedSpec:           spec.ContractDigest(model),
		ReviewedScope:          []string{"file.go"},
		ReviewedMaterialDigest: "old-material",
	}
	ledger := session.New("task", "now").
		WithEntry(failedReview).
		WithEntry(session.Entry{ID: "build-1", Type: "build", Status: string(spec.StatusReview)})

	state := Project(ledger, model, Options{
		Now:             time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC),
		MaterialSeal:    reviewevidence.MaterialSeal{Scope: []string{"file.go"}, Digest: "new-material"},
		HasMaterialSeal: true,
	})
	if state.Kind != KindReviewStaleAfterBuild || !state.CanStartReview || !state.HasReview {
		t.Fatalf("changed repair state = %+v", state)
	}
}

func TestProjectAllowsLegacyReviewRerunWhenMaterialSealIsMissing(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task", Status: spec.StatusReview, Summary: "contract"}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{
			ID:           "review-fail",
			Type:         "review",
			Status:       corereview.VerdictFail,
			Output:       corereview.EncodeDossier(blockingDossierForProjectionTest()),
			ReviewedSpec: spec.ContractDigest(model),
		}).
		WithEntry(session.Entry{ID: "build-1", Type: "build", Status: string(spec.StatusReview)})

	state := Project(ledger, model, Options{Now: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)})
	if state.Kind != KindReviewStaleAfterBuild || !state.CanStartReview {
		t.Fatalf("legacy repair state = %+v", state)
	}
}

func TestProjectDetectsPassingReviewStaleness(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	ledger := session.New("task", "now").WithEntry(sessiontest.PassingReviewEntry("review-1", "codex"))
	cleanModel := spec.Model{TaskID: "task", Status: spec.StatusReview}

	state := Project(ledger, cleanModel, Options{Now: now, WorkspaceSeal: WorkspaceSeal{Head: "head", Dirty: "true", Diff: ledger.Entries[len(ledger.Entries)-1].ReviewedDiff}, HasWorkspaceSeal: true})
	if state.Kind != KindReviewPassed || state.Next != "scafld complete task" {
		t.Fatalf("passing state = %+v", state)
	}

	changedSpec := cleanModel
	changedSpec.Summary = "changed after review"
	state = Project(ledger, changedSpec, Options{Now: now})
	if state.Kind != KindReviewStaleAfterSpec || !state.CanStartReview {
		t.Fatalf("spec stale state = %+v", state)
	}

	state = Project(ledger, cleanModel, Options{Now: now, WorkspaceSeal: WorkspaceSeal{Head: "head", Dirty: "false", Diff: ledger.Entries[len(ledger.Entries)-1].ReviewedDiff}, HasWorkspaceSeal: true})
	if state.Kind != KindReviewStaleAfterWorkspace || !state.CanStartReview {
		t.Fatalf("workspace stale state = %+v", state)
	}
}

func blockingDossierForProjectionTest() corereview.Dossier {
	return corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeVerify,
		Summary: "Review found a blocker.",
		Findings: []corereview.Finding{{
			ID:               "f1",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Category:         "correctness",
			Confidence:       corereview.ConfidenceHigh,
			Location:         &corereview.Location{Path: "file.go", Line: 1},
			Evidence:         "bug evidence",
			Impact:           "bug impact",
			Validation:       "go test ./...",
			Summary:          "bug",
		}},
		AttackLog: []corereview.AttackLogEntry{{Target: "diff", Attack: "scan", Result: corereview.AttackResultFinding}},
	}
}

func TestProjectPrefersMaterialSealOverFullWorkspaceSeal(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	entry := sessiontest.PassingReviewEntry("review-1", "codex")
	entry.ReviewedScope = []string{"api/handler.go"}
	entry.ReviewedMaterialDigest = "same-material"
	ledger := session.New("task", "now").WithEntry(entry)
	model := spec.Model{TaskID: "task", Status: spec.StatusReview}

	state := Project(ledger, model, Options{
		Now:              now,
		WorkspaceSeal:    WorkspaceSeal{Head: "different-head", Dirty: "false", Diff: "different-full-workspace"},
		HasWorkspaceSeal: true,
		MaterialSeal:     reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "same-material"},
		HasMaterialSeal:  true,
	})
	if state.Kind != KindReviewPassed {
		t.Fatalf("material match should keep review passed despite full workspace drift: %+v", state)
	}

	state = Project(ledger, model, Options{
		Now:              now,
		WorkspaceSeal:    WorkspaceSeal{Head: "head", Dirty: "true", Diff: entry.ReviewedDiff},
		HasWorkspaceSeal: true,
		MaterialSeal:     reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "changed-material"},
		HasMaterialSeal:  true,
	})
	if state.Kind != KindReviewStaleAfterWorkspace || state.Reason != "latest review is stale against current task material" {
		t.Fatalf("material mismatch should stale review: %+v", state)
	}
}

func TestProjectCompletedUsesTerminalAuthority(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("review-1", "codex")).
		WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})
	state := Project(ledger, spec.Model{TaskID: "task", Status: spec.StatusCompleted}, Options{Now: time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)})
	if state.Kind != KindCompletedAuthorized || !state.Authority.Valid || !state.HasReview || state.LatestReview.ID != "review-1" {
		t.Fatalf("completed state = %+v", state)
	}
}
