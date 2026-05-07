package complete

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
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

func TestCompleteRequiresPassedReviewGate(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Gate != "complete" || gateErr.Failure.Next != "scafld review task" {
		t.Fatalf("gate error = %#v, ok=%v", gateErr, errors.As(err, &gateErr))
	}
	if specs.model.Status == spec.StatusCompleted {
		t.Fatalf("complete should not mutate spec after failed review")
	}
}

func TestCompleteLifecycleCommandUpdatesSessionAndSpecAfterPassedReview(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "codex"})
	sessions := &fakeSessions{ledger: ledger}
	model, err := Run(context.Background(), specs, sessions, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCompleted || sessions.entry.Status != "completed" {
		t.Fatalf("model=%+v entry=%+v", model, sessions.entry)
	}
}

func TestCompleteRejectsLocalReviewProvider(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "local"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsMissingOrUnknownReviewProvider(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"", "unknown"} {
		specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
		ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: provider})
		_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
		if !errors.Is(err, ErrReviewGate) {
			t.Fatalf("provider %q error = %v, want %v", provider, err, ErrReviewGate)
		}
	}
}

func TestCompleteAcceptsKnownExternalReviewProviders(t *testing.T) {
	t.Parallel()

	for _, provider := range []string{"codex", "claude", "command"} {
		specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
		ledger := session.New("task", "now").WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: provider})
		_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
		if err != nil {
			t.Fatalf("provider %q error = %v", provider, err)
		}
	}
}

func TestCompleteRejectsStaleReviewAfterLaterBuildEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "codex"}).
		WithEntry(session.Entry{Type: "criterion", CriterionID: "ac1", Status: "fail"}).
		WithEntry(session.Entry{Type: "build", Status: string(spec.StatusBlocked), Reason: "acceptance criteria failed"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}

func TestCompleteRejectsStaleReviewAfterLaterReviewAttempt(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "pass"}}}
	ledger := session.New("task", "now").
		WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "codex"}).
		WithEntry(session.Entry{Type: "review_attempt", Status: "failed", Reason: "review provider failed"})
	_, err := Run(context.Background(), specs, &fakeSessions{ledger: ledger}, fakeClock{}, "task")
	if !errors.Is(err, ErrReviewGate) {
		t.Fatalf("error = %v, want %v", err, ErrReviewGate)
	}
}
