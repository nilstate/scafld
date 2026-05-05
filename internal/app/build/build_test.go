package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/execution"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecs struct {
	model spec.Model
	path  string
}

func (f *fakeSpecs) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}
func (f *fakeSpecs) Save(_ context.Context, path string, model spec.Model) error {
	f.path = path
	f.model = model
	return nil
}

type fakeSessions struct{ ledger session.Session }

func (f *fakeSessions) Append(_ context.Context, taskID string, entry session.Entry, now string) (session.Session, error) {
	if f.ledger.TaskID == "" {
		f.ledger = session.New(taskID, now)
	}
	f.ledger = f.ledger.WithEntry(entry)
	return f.ledger, nil
}

func (f *fakeSessions) Load(context.Context, string) (session.Session, error) { return f.ledger, nil }

type fakeRunner struct{ exit int }

func (f fakeRunner) Run(context.Context, execution.Request) (execution.Result, error) {
	return execution.Result{ExitCode: f.exit, Output: "ok"}, nil
}

type fakeBuildClock struct{}

func (fakeBuildClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestPhaseCriterionEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, fakeRunner{}, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Passed != 1 || specs.model.Status != spec.StatusReview {
		t.Fatalf("output %+v model %+v", out, specs.model)
	}
	if specs.model.Phases[0].Status != "completed" {
		t.Fatalf("phase status = %q, want completed", specs.model.Phases[0].Status)
	}
	if specs.model.CurrentState.AllowedFollowUp != "scafld review task" {
		t.Fatalf("next action = %q", specs.model.CurrentState.AllowedFollowUp)
	}
	latest := sessions.ledger.Entries[len(sessions.ledger.Entries)-1]
	if latest.Type != "build" || latest.Status != string(spec.StatusReview) {
		t.Fatalf("latest session entry = %+v, want build review result", latest)
	}
}

func TestBuildBlocksWhenCriterionHasNoEvidence(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, fakeRunner{}, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusBlocked || out.Failed != 1 {
		t.Fatalf("output = %+v, want blocked with one failed/pending criterion", out)
	}
}

func TestBuildRejectsDraft(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusDraft}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, fakeRunner{}, fakeBuildClock{}, Input{TaskID: "task"})
	if !errors.Is(err, ErrSpecNotBuildable) {
		t.Fatalf("error = %v, want %v", err, ErrSpecNotBuildable)
	}
}
