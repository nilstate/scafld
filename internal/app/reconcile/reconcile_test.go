package reconcile

import (
	"context"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecStore struct{ model spec.Model }

func (f *fakeSpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}
func (f *fakeSpecStore) Save(_ context.Context, _ string, model spec.Model) error {
	f.model = model
	return nil
}

type fakeSessionStore struct{ ledger session.Session }

func (f fakeSessionStore) Load(context.Context, string) (session.Session, error) {
	return f.ledger, nil
}

func TestProjectionSourceOfTruth(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecStore{model: spec.Model{TaskID: "task", Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero, Status: "fail"}}}}}}
	sessions := fakeSessionStore{ledger: session.New("task", "now").WithEntry(session.Entry{ID: "e1", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Command: "true", ExpectedKind: string(acceptance.ExpectedExitCodeZero), CriterionType: "command"})}
	model, err := Run(context.Background(), specs, sessions, "task")
	if err != nil {
		t.Fatal(err)
	}
	if model.Phases[0].Acceptance[0].Status != "pass" {
		t.Fatalf("status should come from session, got %+v", model.Phases[0].Acceptance[0])
	}
}
