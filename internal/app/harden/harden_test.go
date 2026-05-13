package harden

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestRunOpensHardenRound(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", Prompt: "prompt body"})
	if err != nil {
		t.Fatal(err)
	}
	if out.HardenStatus != spec.HardenInProgress || out.RoundID != "round-1" {
		t.Fatalf("output = %+v", out)
	}
	if out.NextCommand != "scafld harden fixture-task --mark-passed" {
		t.Fatalf("next command = %q", out.NextCommand)
	}
	if out.Prompt != "prompt body" {
		t.Fatalf("prompt = %q", out.Prompt)
	}
	got := store.model
	if got.HardenStatus != spec.HardenInProgress {
		t.Fatalf("harden status = %s", got.HardenStatus)
	}
	if len(got.HardenRounds) != 1 || got.HardenRounds[0].Status != string(spec.HardenInProgress) || got.HardenRounds[0].StartedAt == "" {
		t.Fatalf("harden rounds = %+v", got.HardenRounds)
	}
	if got.CurrentState.AllowedFollowUp != out.NextCommand {
		t.Fatalf("current state = %+v", got.CurrentState)
	}
}

func TestRunMarkPassedRequiresExistingRound(t *testing.T) {
	t.Parallel()

	_, err := Run(context.Background(), newMemorySpecStore(fixtureModel()), fixedClock{}, Input{
		TaskID:     "fixture-task",
		MarkPassed: true,
	})
	if !errors.Is(err, ErrNoHardenRound) {
		t.Fatalf("err = %v, want %v", err, ErrNoHardenRound)
	}
}

func TestRunMarkPassedClosesLatestRound(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{
		{ID: "round-1", Status: string(spec.HardenFailed)},
		{ID: "round-2", Status: string(spec.HardenInProgress), Checks: passedChecks()},
	}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !out.MarkedPassed || out.RoundID != "round-2" || out.HardenStatus != spec.HardenPassed {
		t.Fatalf("output = %+v", out)
	}
	if got := store.model.HardenRounds[1].Status; got != string(spec.HardenPassed) {
		t.Fatalf("latest round status = %s", got)
	}
	if store.model.HardenRounds[0].Status != string(spec.HardenFailed) {
		t.Fatalf("previous round was changed: %+v", store.model.HardenRounds)
	}
	if store.model.CurrentState.AllowedFollowUp != "scafld approve fixture-task" {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
	if store.model.HardenRounds[1].EndedAt == "" {
		t.Fatalf("ended_at was not recorded: %+v", store.model.HardenRounds[1])
	}
}

func TestRunRejectsNonDraftSpec(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	store.path = filepath.Join(t.TempDir(), ".scafld", "specs", "approved", "fixture-task.md")
	_, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task"})
	if !errors.Is(err, ErrSpecNotDraft) {
		t.Fatalf("err = %v, want %v", err, ErrSpecNotDraft)
	}
}

func TestRunMarkPassedRejectsUngroundedQuestions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: passedChecks(),
		Questions: []spec.HardenQuestion{
			{Question: "Who owns this?", GroundedIn: "code:code.go:1", RecommendedAnswer: "Use the owner.", AnsweredWith: "Use the owner."},
			{Question: "What proves this?", GroundedIn: "code:missing.go:1", RecommendedAnswer: "Use missing.", AnsweredWith: "Use missing."},
			{Question: "Why now?", RecommendedAnswer: "Because scope requires it.", AnsweredWith: "Keep scope."},
			{Question: "Where?", GroundedIn: "code:code.go", RecommendedAnswer: "Use code.go.", AnsweredWith: "Use code.go."},
		},
	}}
	store := newMemorySpecStore(model)
	store.path = filepath.Join(root, ".scafld", "specs", "drafts", "fixture-task.md")
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true, Root: root})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || gateErr.Failure.Gate != "harden" || gateErr.Failure.Expected == "" {
		t.Fatalf("gate error = %#v", gateErr)
	}
	if len(out.Warnings) != 3 {
		t.Fatalf("warnings = %+v, want 3", out.Warnings)
	}
	joined := strings.Join(out.Warnings, "\n")
	if !strings.Contains(joined, "missing.go") ||
		!strings.Contains(joined, "missing grounded_in") ||
		!strings.Contains(joined, "expected code:<path>:<line>") ||
		!strings.Contains(joined, "code:src/file.go:42") {
		t.Fatalf("warnings did not explain citation issues: %+v", out.Warnings)
	}
	if store.model.HardenStatus == spec.HardenPassed {
		t.Fatalf("hardening passed despite invalid citations: %+v", store.model.HardenRounds)
	}
}

func TestRunMarkPassedRejectsMissingHardenChecks(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Questions: []spec.HardenQuestion{{
			Question:          "Is this ready?",
			GroundedIn:        "spec_gap:Scope",
			RecommendedAnswer: "No until checks exist.",
			AnsweredWith:      "Add checks.",
		}},
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if len(out.Warnings) == 0 || !strings.Contains(strings.Join(out.Warnings, "\n"), "missing harden checks") {
		t.Fatalf("warnings did not require harden checks: %+v", out.Warnings)
	}
}

func TestRunMarkPassedRejectsOpenQuestions(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: passedChecks(),
		Questions: []spec.HardenQuestion{{
			Question:   "Who owns this?",
			GroundedIn: "spec_gap:Scope",
		}},
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	joined := strings.Join(out.Warnings, "\n")
	if !strings.Contains(joined, "missing recommended answer") || !strings.Contains(joined, "missing answered with resolution") {
		t.Fatalf("warnings did not reject open question: %+v", out.Warnings)
	}
}

func TestRunMarkPassedAllowsNotApplicableChecks(t *testing.T) {
	t.Parallel()

	checks := passedChecks()
	checks[4] = spec.HardenCheck{Name: "Rollback/repair audit", GroundedIn: "spec_gap:Rollback", Result: "not_applicable", Evidence: "Docs-only change with no runtime rollback."}
	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: checks,
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !out.MarkedPassed {
		t.Fatalf("output = %+v", out)
	}
}

func passedChecks() []spec.HardenCheck {
	return []spec.HardenCheck{
		{Name: "Path audit", GroundedIn: "spec_gap:Scope", Result: "passed", Evidence: "Paths checked."},
		{Name: "Command audit", GroundedIn: "spec_gap:Validation", Result: "passed", Evidence: "Commands checked."},
		{Name: "Scope/migration audit", GroundedIn: "spec_gap:Risks", Result: "passed", Evidence: "Migration claims checked."},
		{Name: "Acceptance timing audit", GroundedIn: "spec_gap:Phases", Result: "passed", Evidence: "Criteria timing checked."},
		{Name: "Rollback/repair audit", GroundedIn: "spec_gap:Rollback", Result: "passed", Evidence: "Rollback checked."},
		{Name: "Design challenge", GroundedIn: "spec_gap:Summary", Result: "passed", Evidence: "Plan is not a bandaid or future compatibility layer."},
	}
}

type memorySpecStore struct {
	model spec.Model
	path  string
}

func newMemorySpecStore(model spec.Model) *memorySpecStore {
	return &memorySpecStore{model: model, path: "/tmp/root/.scafld/specs/drafts/fixture-task.md"}
}

func (s *memorySpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return s.model, s.path, nil
}

func (s *memorySpecStore) Save(_ context.Context, path string, model spec.Model) error {
	s.path = path
	s.model = model
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
}

func fixtureModel() spec.Model {
	return spec.Model{
		Version:      "2.0",
		TaskID:       "fixture-task",
		Created:      "2026-05-04T00:00:00Z",
		Updated:      "2026-05-04T00:00:00Z",
		Title:        "Fixture task",
		Status:       spec.StatusDraft,
		HardenStatus: spec.HardenNotRun,
		Size:         spec.SizeSmall,
		RiskLevel:    spec.RiskLow,
		CurrentState: spec.CurrentState{AllowedFollowUp: "scafld approve fixture-task"},
	}
}
