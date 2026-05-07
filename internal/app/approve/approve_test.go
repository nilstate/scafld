package approve

import (
	"context"
	"errors"
	"strings"
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

type fakeSessions struct{ entries []session.Entry }

func (f *fakeSessions) Append(_ context.Context, _ string, entry session.Entry, _ string) (session.Session, error) {
	f.entries = append(f.entries, entry)
	ledger := session.New("task", "now")
	for _, item := range f.entries {
		ledger = ledger.WithEntry(item)
	}
	return ledger, nil
}

type fakeWorkspace struct{ snapshot []string }

func (f fakeWorkspace) ChangedFiles(context.Context) ([]string, error) {
	return append([]string(nil), f.snapshot...), nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestApproveRequiresDraft(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeClock{}, "task")
	if !errors.Is(err, ErrSpecNotDraft) {
		t.Fatalf("error = %v, want %v", err, ErrSpecNotDraft)
	}
}

func TestApproveAppendsSessionBeforeSavingSpec(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusDraft}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, fakeWorkspace{snapshot: []string{" M hash preexisting.go"}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusApproved || specs.model.Status != spec.StatusApproved {
		t.Fatalf("output=%+v model=%+v", out, specs.model)
	}
	if len(sessions.entries) != 2 ||
		sessions.entries[0].Type != session.EntryWorkspaceBaseline ||
		sessions.entries[0].Status != "captured" ||
		!strings.Contains(sessions.entries[0].Output, "preexisting.go") ||
		sessions.entries[1].Type != "approval" ||
		sessions.entries[1].Status != "approved" {
		t.Fatalf("entries = %+v", sessions.entries)
	}
}
