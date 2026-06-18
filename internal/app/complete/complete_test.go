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
