package complete

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
	"github.com/nilstate/scafld/v2/internal/testkit/sessiontest"
)

type fakeSpecs struct{ model spec.Model }

func (f *fakeSpecs) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}

func (f *fakeSpecs) Save(_ context.Context, _ string, model spec.Model) error {
	f.model = model
	return nil
}

type fakeSessions struct {
	ledger session.Session
	entry  session.Entry
}

func (f *fakeSessions) Append(_ context.Context, taskID string, entry session.Entry, now string) (session.Session, error) {
	if f.ledger.TaskID == "" {
		f.ledger = session.New(taskID, now)
	}
	f.entry = entry
	f.ledger = f.ledger.WithEntry(entry)
	return f.ledger, nil
}

func (f *fakeSessions) Load(context.Context, string) (session.Session, error) {
	return f.ledger, nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

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

func (f fakeWorkspace) MaterialSeal(_ context.Context, scope []string) (reviewevidence.MaterialSeal, error) {
	if f.material.Digest != "" {
		return f.material, nil
	}
	normalized := coreworkspace.NormalizeScope(scope)
	filtered := coreworkspace.Filter(reviewevidence.ComparisonSnapshot(f.snapshot), normalized)
	files := make([]reviewevidence.MaterialFile, 0, len(filtered))
	for _, raw := range filtered {
		change := coreworkspace.ParseChange(raw)
		if change.Path == "" || change.Fingerprint == "" {
			continue
		}
		files = append(files, reviewevidence.MaterialFile{Path: change.Path, SHA256: change.Fingerprint})
	}
	return reviewevidence.MaterialSeal{Scope: normalized, Digest: reviewevidence.MaterialDigest(normalized, files)}, nil
}

func sealedExternalReviewEntryForWorkspace(id string, provider string, workspace fakeWorkspace) session.Entry {
	entry := sessiontest.PassingReviewEntry(id, provider)
	snapshot := reviewevidence.ComparisonSnapshot(workspace.snapshot)
	if workspace.head == "" {
		entry.ReviewedHead = "unborn"
	} else {
		entry.ReviewedHead = workspace.head
	}
	entry.ReviewedDirty = reviewevidence.SnapshotDirty(snapshot)
	entry.ReviewedDiff = reviewevidence.SnapshotDigest(snapshot)
	return entry
}

func sealedExternalReviewEntryForMaterial(id string, provider string, workspace fakeWorkspace, material reviewevidence.MaterialSeal) session.Entry {
	entry := sealedExternalReviewEntryForWorkspace(id, provider, workspace)
	entry.ReviewedScope = append([]string(nil), material.Scope...)
	entry.ReviewedMaterialDigest = material.Digest
	return entry
}

func TestCompleteRequiresPassedReviewGate(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{
		TaskID:       "task",
		Status:       spec.StatusReview,
		Review:       spec.ReviewState{Status: "completed", Verdict: "pass"},
		CurrentState: spec.CurrentState{AllowedFollowUp: "scafld handoff task"},
	}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Gate != "complete" || gateErr.Failure.Next != "scafld handoff task" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
	if specs.model.Status == spec.StatusCompleted {
		t.Fatalf("complete should not mutate spec after failed review")
	}
}

func TestCompleteRejectionDoesNotPointBackToCompleteAfterLaterReviewAttempt(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{
		TaskID:       "task",
		Status:       spec.StatusReview,
		Review:       spec.ReviewState{Status: "completed", Verdict: "pass"},
		CurrentState: spec.CurrentState{AllowedFollowUp: "scafld complete task"},
	}}
	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("", "codex")).
		WithEntry(session.Entry{Type: "review_attempt", Status: "failed", Reason: "provider failed"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Next != "scafld handoff task" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteLifecycleCommandUpdatesSessionAndSpecAfterPassedReview(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(sessiontest.PassingReviewEntry("", "codex"))
	sessions := &fakeSessions{ledger: ledger}
	model, err := Run(context.Background(), specs, sessions, nil, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCompleted || sessions.entry.Status != "completed" {
		t.Fatalf("model=%+v entry=%+v", model, sessions.entry)
	}
}

func TestCompleteRejectsReviewSealWhenWorkspaceChangedAfterReview(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	reviewWorkspace := fakeWorkspace{head: "head", snapshot: nil}
	currentWorkspace := fakeWorkspace{head: "head", snapshot: []string{" M changed file.go"}}
	ledger := session.New("task", "now").WithEntry(sealedExternalReviewEntryForWorkspace("", "codex", reviewWorkspace))
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, currentWorkspace, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Reason != "latest review is stale against current workspace" || gateErr.Failure.Actual == "" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteAcceptsMaterialSealDespiteAmbientWorkspaceChange(t *testing.T) {
	t.Parallel()

	material := reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "same-material"}
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	reviewWorkspace := fakeWorkspace{head: "head", snapshot: []string{" M reviewed api/handler.go"}}
	currentWorkspace := fakeWorkspace{head: "new-head", snapshot: []string{" M adjacent docs/readme.md"}, material: material}
	ledger := session.New("task", "now").WithEntry(sealedExternalReviewEntryForMaterial("", "codex", reviewWorkspace, material))
	model, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, currentWorkspace, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCompleted {
		t.Fatalf("model status = %s, want completed", model.Status)
	}
}

func TestCompleteRejectsReviewSealWhenTaskMaterialChanged(t *testing.T) {
	t.Parallel()

	reviewedMaterial := reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "reviewed-material"}
	currentMaterial := reviewevidence.MaterialSeal{Scope: []string{"api/handler.go"}, Digest: "changed-material"}
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	reviewWorkspace := fakeWorkspace{head: "head", snapshot: []string{" M reviewed api/handler.go"}}
	currentWorkspace := fakeWorkspace{head: "head", snapshot: []string{" M changed api/handler.go"}, material: currentMaterial}
	ledger := session.New("task", "now").WithEntry(sealedExternalReviewEntryForMaterial("", "codex", reviewWorkspace, reviewedMaterial))
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, currentWorkspace, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Reason != "latest review is stale against current task material" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteRejectsReviewSealWhenSpecContractChangedAfterReview(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Summary: "changed after review", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(sessiontest.PassingReviewEntry("", "codex"))
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Reason != "latest review is stale against current spec" || !strings.Contains(gateErr.Failure.Actual, "reviewed_spec") {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteRejectsLocalReviewProvider(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "local"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsMissingOrUnknownReviewProvider(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"", "unknown"} {
		specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
		ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: provider})
		_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
		if !errors.Is(err, ErrReviewGate) {
			t.Fatalf("provider %q error = %v, want %v", provider, err, ErrReviewGate)
		}
	}
}

func TestCompleteAcceptsKnownExternalReviewProviders(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"codex", "claude", "gemini", "command"} {
		specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
		ledger := session.New("task", "now").WithEntry(sessiontest.PassingReviewEntry("", provider))
		_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
		if err != nil {
			t.Fatalf("provider %q error = %v", provider, err)
		}
	}
}

func TestCompleteRejectsUnsealedExternalReview(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "codex", Output: corereview.EncodeDossier(corereview.Dossier{
		Verdict:  corereview.VerdictPass,
		Mode:     corereview.ModeDiscover,
		Provider: "codex",
		Summary:  "clean",
		AttackLog: []corereview.AttackLogEntry{{
			Target: "diff",
			Attack: "scan",
			Result: corereview.AttackResultClean,
		}},
	})})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Reason != "latest review packet is missing" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteRejectsTamperedReviewPacketHash(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	entry := sessiontest.PassingReviewEntry("", "codex")
	entry.CanonicalResponseSHA256 = "wrong"
	ledger := session.New("task", "now").WithEntry(entry)
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Reason != "latest review packet hash mismatch" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteAcceptsAuditedHumanReviewProvider(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{Type: "review_override", Status: "accepted", Provider: "human", Reason: "operator reviewed PR 123"}).
		WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "human"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompleteRejectsUnauditedHumanReviewProvider(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "human"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsStaleReviewAfterLaterBuildEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("", "codex")).
		WithEntry(session.Entry{Type: "criterion", CriterionID: "ac1", Status: "fail"}).
		WithEntry(session.Entry{Type: "build", Status: string(spec.StatusBlocked), Reason: "acceptance criteria failed"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsStaleReviewAfterLaterReviewAttempt(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("", "codex")).
		WithEntry(session.Entry{Type: "review_attempt", Status: "failed", Reason: "review provider failed"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsOlderPassAfterStaleRunningReviewAttempt(t *testing.T) {
	t.Parallel()

	recordedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC).Add(-3 * time.Hour).Format(time.RFC3339)
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(sessiontest.PassingReviewEntry("", "codex")).
		WithEntry(session.Entry{Type: "review_attempt", Status: "running", RecordedAt: recordedAt, Reason: "provider wedged"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Next != "scafld review task" || gateErr.Failure.Reason != "running review attempt lease expired" {
		t.Fatalf("gate error = %#v", gateErr)
	}
}

func TestCompleteRejectsPassingReviewAfterBlockingReviewWithoutRepairEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{ID: "review-fail", Type: "review", Status: corereview.VerdictFail, Provider: "codex"}).
		WithEntry(sessiontest.PassingReviewEntry("review-pass", "codex"))
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Actual != "latest passing review follows review-fail without changed workspace or build evidence" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
}

func TestCompleteAcceptsPassingReviewAfterChangedAttemptEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{ID: "attempt-fail", Type: "review_attempt", Status: "running", Output: "task_changes_since_baseline:\n- changed file.go (old)"}).
		WithEntry(session.Entry{ID: "review-fail", Type: "review", Status: corereview.VerdictFail, Provider: "codex"}).
		WithEntry(session.Entry{ID: "attempt-pass", Type: "review_attempt", Status: "running", Output: "task_changes_since_baseline:\n- changed file.go (new)"}).
		WithEntry(sessiontest.PassingReviewEntry("review-pass", "codex"))
	model, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, nil, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCompleted {
		t.Fatalf("status = %q, want completed", model.Status)
	}
}
