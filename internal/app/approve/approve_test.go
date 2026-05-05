package approve

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

func (f *fakeSpecs) Find(string) (string, error) {
	return "approved/task.md", nil
}

type fakeSessions struct{ entry session.Entry }

func (f *fakeSessions) Append(_ context.Context, _ string, entry session.Entry, _ string) (session.Session, error) {
	f.entry = entry
	return session.New("task", "now").WithEntry(entry), nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestApproveRequiresDraft(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, fakeClock{}, "task")
	if !errors.Is(err, ErrSpecNotDraft) {
		t.Fatalf("error = %v, want %v", err, ErrSpecNotDraft)
	}
}

func TestApproveAppendsSessionBeforeSavingSpec(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusDraft}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusApproved || specs.model.Status != spec.StatusApproved {
		t.Fatalf("output=%+v model=%+v", out, specs.model)
	}
	if sessions.entry.Type != "approval" || sessions.entry.Status != "approved" {
		t.Fatalf("entry = %+v", sessions.entry)
	}
}
