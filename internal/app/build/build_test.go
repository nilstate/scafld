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

type fakeRunner struct {
	exit         int
	exitBy       map[string]int
	commands     []string
	env          []string
	timeouts     []time.Duration
	idleTimeouts []time.Duration
}

func (f *fakeRunner) Run(_ context.Context, req execution.Request) (execution.Result, error) {
	f.commands = append(f.commands, req.Command)
	f.env = append([]string(nil), req.Env...)
	f.timeouts = append(f.timeouts, req.Timeout)
	f.idleTimeouts = append(f.idleTimeouts, req.IdleTimeout)
	exit := f.exit
	if f.exitBy != nil {
		exit = f.exitBy[req.Command]
	}
	return execution.Result{ExitCode: exit, Output: "ok"}, nil
}

type fakeWorkspace struct{ snapshot []string }

func (f fakeWorkspace) ChangedFiles(context.Context) ([]string, error) {
	return append([]string(nil), f.snapshot...), nil
}

type fakeBuildClock struct{}

func (fakeBuildClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestBuildOpensPhaseWithoutRunningAcceptance(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "false", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Passed != 0 || out.Failed != 0 || specs.model.Status != spec.StatusActive {
		t.Fatalf("output %+v model %+v", out, specs.model)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("first build should not run future acceptance, commands = %+v", runner.commands)
	}
	if specs.model.Phases[0].Status != "active" || specs.model.CurrentState.CurrentPhase != "phase1" {
		t.Fatalf("phase state = %+v current=%q, want active phase1", specs.model.Phases[0], specs.model.CurrentState.CurrentPhase)
	}
	if specs.model.CurrentState.AllowedFollowUp != "scafld handoff task" {
		t.Fatalf("next action = %q", specs.model.CurrentState.AllowedFollowUp)
	}
	latest := sessions.ledger.Entries[len(sessions.ledger.Entries)-1]
	if latest.Type != "build" || latest.Status != string(spec.StatusActive) {
		t.Fatalf("latest session entry = %+v, want build active result", latest)
	}
}

func TestBuildRunsOpenedPhaseAndMovesToReview(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	sessions := &fakeSessions{}
	runner := &fakeRunner{}
	if _, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"}); err != nil {
		t.Fatal(err)
	}
	out, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Passed != 1 || out.Failed != 0 || specs.model.Status != spec.StatusReview {
		t.Fatalf("output %+v model %+v", out, specs.model)
	}
	if len(runner.commands) != 1 || runner.commands[0] != "true" {
		t.Fatalf("commands = %+v, want one phase command", runner.commands)
	}
	if specs.model.Phases[0].Status != "completed" {
		t.Fatalf("phase status = %q, want completed", specs.model.Phases[0].Status)
	}
	if specs.model.CurrentState.AllowedFollowUp != "scafld review task" {
		t.Fatalf("next action = %q", specs.model.CurrentState.AllowedFollowUp)
	}
}

func TestBuildBlocksOnlyAfterEvidenceAttempt(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "false", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	runner := &fakeRunner{exitBy: map[string]int{"false": 1, "true": 0}}
	sessions := &fakeSessions{}
	started, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if started.Status != spec.StatusActive || len(runner.commands) != 0 {
		t.Fatalf("first build = %+v commands=%+v, want active without evidence", started, runner.commands)
	}
	out, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusBlocked {
		t.Fatalf("output = %+v, want blocked", out)
	}
	if len(runner.commands) != 1 || runner.commands[0] != "false" {
		t.Fatalf("commands = %+v, want failed phase command only", runner.commands)
	}
	if specs.model.Phases[0].Status != "blocked" {
		t.Fatalf("phase1 status = %q, want blocked", specs.model.Phases[0].Status)
	}
	if specs.model.CurrentState.Blockers == "" || specs.model.CurrentState.Blockers == "none" {
		t.Fatalf("blocked state should describe blockers, got %q", specs.model.CurrentState.Blockers)
	}
	if out.Next != "scafld handoff task" || specs.model.CurrentState.AllowedFollowUp != "scafld handoff task" {
		t.Fatalf("blocked next action = %+v model=%+v", out, specs.model.CurrentState)
	}
}

func TestBuildAdvancesOnePhasePerInvocation(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{
		TaskID: "task",
		Status: spec.StatusApproved,
		Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{
			ID:           "global",
			Command:      "go test ./...",
			ExpectedKind: acceptance.ExpectedExitCodeZero,
		}}},
		Phases: []spec.Phase{
			{ID: "phase1", Name: "First", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "phase1", ExpectedKind: acceptance.ExpectedExitCodeZero}}},
			{ID: "phase2", Name: "Second", Acceptance: []spec.Criterion{{ID: "ac2", PhaseID: "phase2", Command: "phase2", ExpectedKind: acceptance.ExpectedExitCodeZero}}},
		},
	}}
	runner := &fakeRunner{}
	sessions := &fakeSessions{}
	if _, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"}); err != nil {
		t.Fatal(err)
	}
	out, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusActive || specs.model.CurrentState.CurrentPhase != "phase2" {
		t.Fatalf("after phase1 output=%+v current=%q", out, specs.model.CurrentState.CurrentPhase)
	}
	if len(runner.commands) != 1 || runner.commands[0] != "phase1" {
		t.Fatalf("commands after phase1 = %+v", runner.commands)
	}
	out, err = Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusReview {
		t.Fatalf("after phase2 output=%+v, want review", out)
	}
	if len(runner.commands) != 3 || runner.commands[1] != "phase2" || runner.commands[2] != "go test ./..." {
		t.Fatalf("commands = %+v, want phase2 then final acceptance", runner.commands)
	}
}

func TestBuildCapturesBaselineOnlyWhenSessionLacksOne(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "true", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	sessions := &fakeSessions{}
	_, err := Run(context.Background(), specs, sessions, fakeWorkspace{snapshot: []string{" M hash preexisting.go"}}, &fakeRunner{}, fakeBuildClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions.ledger.Entries) == 0 || sessions.ledger.Entries[0].Type != session.EntryWorkspaceBaseline || sessions.ledger.Entries[0].Output != " M hash preexisting.go" {
		t.Fatalf("baseline was not first session entry: %+v", sessions.ledger.Entries)
	}
}

func TestBuildPassesExecutionEnvToCriteria(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "make api-test", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	runner := &fakeRunner{}
	env := []string{"BUNDLE_GEMFILE=api/Gemfile", "PATH=/tmp/rbenv/shims:/usr/bin"}
	sessions := &fakeSessions{}
	if _, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task", Env: env}); err != nil {
		t.Fatal(err)
	}
	out, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, Input{TaskID: "task", Env: env})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusReview {
		t.Fatalf("output = %+v, want review", out)
	}
	if len(runner.env) != len(env) || runner.env[0] != env[0] || runner.env[1] != env[1] {
		t.Fatalf("runner env = %+v, want %+v", runner.env, env)
	}
}

func TestBuildPassesConfiguredTimeoutsToCriteria(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved, Phases: []spec.Phase{{ID: "phase1", Name: "Phase", Acceptance: []spec.Criterion{{ID: "ac1", PhaseID: "phase1", Command: "make check", ExpectedKind: acceptance.ExpectedExitCodeZero}}}}}}
	runner := &fakeRunner{}
	sessions := &fakeSessions{}
	input := Input{TaskID: "task", Timeout: 2 * time.Minute, IdleTimeout: 30 * time.Second}
	if _, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, input); err != nil {
		t.Fatal(err)
	}
	if _, err := Run(context.Background(), specs, sessions, nil, runner, fakeBuildClock{}, input); err != nil {
		t.Fatal(err)
	}
	if len(runner.timeouts) != 1 || runner.timeouts[0] != 2*time.Minute || runner.idleTimeouts[0] != 30*time.Second {
		t.Fatalf("timeouts = %+v idle=%+v", runner.timeouts, runner.idleTimeouts)
	}
}

func TestBuildRejectsDraft(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusDraft}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, &fakeRunner{}, fakeBuildClock{}, Input{TaskID: "task"})
	if !errors.Is(err, ErrSpecNotBuildable) {
		t.Fatalf("error = %v, want %v", err, ErrSpecNotBuildable)
	}
}
