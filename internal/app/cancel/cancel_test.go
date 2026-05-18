package cancel

import (
	"context"
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

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestCancelLifecycleCommandUpdatesSessionAndSpec(t *testing.T) {
	t.Parallel()
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusActive}}
	sessions := &fakeSessions{}
	model, err := Run(context.Background(), specs, sessions, fakeClock{}, "task", "stop")
	if err != nil {
		t.Fatal(err)
	}
	if model.Status != spec.StatusCancelled || sessions.entry.Status != "cancelled" {
		t.Fatalf("model=%+v entry=%+v", model, sessions.entry)
	}
	if model.CurrentState.Next != "done" || model.CurrentState.AllowedFollowUp != "none" || model.CurrentState.Blockers != "none" {
		t.Fatalf("current state = %+v", model.CurrentState)
	}
}
