package complete

import (
	"context"
	"errors"
	"testing"
	"time"

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
