package plan

import (
	"context"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeStore struct{ model spec.Model }
type fakeClock struct{}

func (f *fakeStore) CreateDraft(_ context.Context, model spec.Model) (string, error) {
	f.model = model
	return "/tmp/task.md", nil
}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestContextErrorAndPorts(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	out, err := Run(context.Background(), store, fakeClock{}, Input{TaskID: "task", Command: "true"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusDraft || store.model.TaskID != "task" {
		t.Fatalf("unexpected output %+v model %+v", out, store.model)
	}
}
