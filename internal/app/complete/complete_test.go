package complete

import (
	"context"
	"errors"
	"testing"
	"time"

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

type fakeSessions struct{ entry session.Entry }

func (f *fakeSessions) Append(_ context.Context, _ string, entry session.Entry, _ string) (session.Session, error) {
	f.entry = entry
	return session.New("task", "now").WithEntry(entry), nil
}

func (f *fakeSessions) Load(context.Context, string) (session.Session, error) {
	return session.New("task", "now").WithEntry(f.entry), nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestCompleteRequiresPassedReviewGate(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusReview, Review: spec.ReviewState{Status: "completed", Verdict: "fail"}}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, fakeClock{}, "task")
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
	sessions := &fakeSessions{}
	model, err := Run(context.Background(), specs, sessions, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCompleted || sessions.entry.Status != "completed" {
		t.Fatalf("model=%+v entry=%+v", model, sessions.entry)
	}
}
