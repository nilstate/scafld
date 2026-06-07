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
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
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
	round := store.model.HardenRounds[0]
	if len(round.Observations) != len(requiredHardenDimensions) {
		t.Fatalf("observation skeleton = %+v", round.Observations)
	}
	for i, observation := range round.Observations {
		if observation.Dimension != requiredHardenDimensions[i] || observation.Anchor != "" || observation.Result != "" || observation.Note != "" {
			t.Fatalf("observation skeleton[%d] = %+v", i, observation)
		}
	}
	if store.model.CurrentState.AllowedFollowUp != out.NextCommand {
		t.Fatalf("current state = %+v", store.model.CurrentState)
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
		{ID: "round-1", Status: string(spec.HardenNeedsRevision)},
		{ID: "round-2", Status: string(spec.HardenInProgress), Observations: passingObservations()},
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
	if store.model.HardenRounds[0].Status != string(spec.HardenNeedsRevision) {
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

func TestRunMarkPassedRejectsInvalidObservationAnchors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "code.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	observations := passingObservations()
	observations[0].Anchor = "code:code.go:1"
	observations[1].Anchor = "code:missing.go:1"
	observations[2].Anchor = ""
	observations[3].Anchor = "code:code.go"
	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Observations: observations,
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
	joined := strings.Join(out.Warnings, "\n")
	if !strings.Contains(joined, "missing.go") ||
		!strings.Contains(joined, "missing anchor") ||
		!strings.Contains(joined, "expected code:<path>:<line>") ||
		!strings.Contains(joined, "code:src/file.go:42") {
		t.Fatalf("warnings did not explain citation issues: %+v", out.Warnings)
	}
	if store.model.HardenStatus == spec.HardenPassed {
		t.Fatalf("hardening passed despite invalid citations: %+v", store.model.HardenRounds)
	}
}

func TestRunMarkPassedRejectsUnknownSpecGapAnchor(t *testing.T) {
	t.Parallel()

	observations := passingObservations()
	observations[0].Anchor = "spec_gap:DefinitelyNotASection"
	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Observations: observations,
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if !strings.Contains(strings.Join(out.Warnings, "\n"), "known spec field") {
		t.Fatalf("warnings = %+v", out.Warnings)
	}
}

func TestRunMarkPassedRejectsMissingObservations(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if len(out.Warnings) == 0 || !strings.Contains(strings.Join(out.Warnings, "\n"), "missing harden observations") {
		t.Fatalf("warnings did not require harden observations: %+v", out.Warnings)
	}
}

func TestRunMarkPassedRejectsOpenBlockingObservation(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	observations := passingObservations()
	observations[5] = spec.HardenObservation{
		Dimension: "design",
		Result:    coreharden.ResultBlocks,
		Anchor:    "spec_gap:Summary",
		Note:      "Plan treats a symptom.",
		Status:    coreharden.StatusOpen,
	}
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Observations: observations,
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if !strings.Contains(strings.Join(out.Warnings, "\n"), "blocking observation is still open") {
		t.Fatalf("warnings = %+v", out.Warnings)
	}
}

func TestRunMarkPassedAllowsAdvisoryObservation(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	observations := passingObservations()
	observations[4] = spec.HardenObservation{
		Dimension: "rollback",
		Result:    coreharden.ResultAdvisory,
		Anchor:    "spec_gap:Rollback",
		Note:      "Rollback could name a recovery command.",
	}
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Observations: observations,
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !out.MarkedPassed || out.HardenStatus != spec.HardenPassed {
		t.Fatalf("output = %+v", out)
	}
}

func TestRunProviderHardenRecordsPassingDossier(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Prompt:   "prompt",
		Provider: fakeHardenProvider{dossier: passingHardenDossier()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HardenStatus != spec.HardenPassed || out.Verdict != coreharden.VerdictPass || out.NextCommand != "scafld approve fixture-task" {
		t.Fatalf("output = %+v", out)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenPassed) || round.Provider != "codex" || round.Model != "gpt-test" || round.Summary == "" || len(round.Observations) != 6 {
		t.Fatalf("round = %+v", round)
	}
}

func TestRunProviderHardenRecordsNeedsRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[5] = coreharden.Observation{
		Dimension: "design",
		Result:    coreharden.ResultBlocks,
		Anchor:    "spec_gap:Summary",
		Note:      "The plan may be future bloat.",
		Default:   "Do not add it until duplicated complexity is proven.",
		Status:    coreharden.StatusOpen,
	}
	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HardenStatus != spec.HardenNeedsRevision || out.Verdict != coreharden.VerdictNeedsRevision {
		t.Fatalf("output = %+v", out)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenNeedsRevision) || len(round.Observations) != 6 {
		t.Fatalf("round = %+v", round)
	}
	if !strings.Contains(store.model.CurrentState.Blockers, "unresolved blocking observation") || !strings.Contains(store.model.CurrentState.AllowedFollowUp, "--provider <provider>") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenAdvisoryObservationsDoNotBlock(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[4] = coreharden.Observation{
		Dimension: "rollback",
		Result:    coreharden.ResultAdvisory,
		Anchor:    "spec_gap:Rollback",
		Note:      "The rollback section could name a more convenient recovery command.",
		Default:   "Use the package's existing recovery command if one exists.",
	}
	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.HardenStatus != spec.HardenPassed || out.Verdict != coreharden.VerdictPass {
		t.Fatalf("output = %+v", out)
	}
	if store.model.CurrentState.AllowedFollowUp != "scafld approve fixture-task" {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenRejectsUnverifiedAnchor(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[0].Anchor = "code:missing.go:1"
	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
		Root:     t.TempDir(),
	})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if out.HardenStatus != spec.HardenError || !strings.Contains(strings.Join(out.Warnings, "\n"), "missing.go") {
		t.Fatalf("output = %+v", out)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || !strings.Contains(round.Summary, "invalid provider dossier evidence") {
		t.Fatalf("round = %+v", round)
	}
}

func TestRunProviderHardenRejectsUnknownSpecGapAnchor(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[0].Anchor = "spec_gap:DefinitelyNotASection"
	store := newMemorySpecStore(fixtureModel())
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
	})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if out.HardenStatus != spec.HardenError || !strings.Contains(strings.Join(out.Warnings, "\n"), "known spec field") {
		t.Fatalf("output = %+v", out)
	}
}

func TestRunProviderHardenClosesRoundOnProviderError(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{err: errors.New("provider unavailable")},
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || round.EndedAt == "" || !strings.Contains(round.Summary, "provider unavailable") {
		t.Fatalf("round = %+v", round)
	}
	if !strings.Contains(store.model.CurrentState.AllowedFollowUp, "--provider <provider>") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenClosesRoundOnInvalidDossier(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: coreharden.Dossier{Summary: "missing observations"}},
	})
	if err == nil {
		t.Fatal("expected invalid dossier error")
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || round.EndedAt == "" || !strings.Contains(round.Summary, "invalid provider dossier") {
		t.Fatalf("round = %+v", round)
	}
}

func TestRunProviderHardenReportsFailureRecordingError(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	store.failSaveAt = 2
	store.saveErr = errors.New("disk full")
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{err: errors.New("provider unavailable")},
	})
	if err == nil {
		t.Fatal("expected joined error")
	}
	text := err.Error()
	if !strings.Contains(text, "provider unavailable") || !strings.Contains(text, "record provider harden failure") || !strings.Contains(text, "disk full") {
		t.Fatalf("error = %v", err)
	}
}

func passingObservations() []spec.HardenObservation {
	return []spec.HardenObservation{
		{Dimension: "path", Result: coreharden.ResultClean, Anchor: "spec_gap:Scope", Note: "Paths checked."},
		{Dimension: "command", Result: coreharden.ResultClean, Anchor: "spec_gap:Validation", Note: "Commands checked."},
		{Dimension: "scope", Result: coreharden.ResultClean, Anchor: "spec_gap:Risks", Note: "Migration claims checked."},
		{Dimension: "timing", Result: coreharden.ResultClean, Anchor: "spec_gap:Phases", Note: "Criteria timing checked."},
		{Dimension: "rollback", Result: coreharden.ResultNotApplicable, Anchor: "spec_gap:Rollback", Note: "Docs-only change has no runtime rollback."},
		{Dimension: "design", Result: coreharden.ResultClean, Anchor: "spec_gap:Summary", Note: "Plan is not a bandaid or future compatibility layer."},
	}
}

type memorySpecStore struct {
	model spec.Model
	path  string
	saves int

	failSaveAt int
	saveErr    error
}

func newMemorySpecStore(model spec.Model) *memorySpecStore {
	return &memorySpecStore{model: model, path: "/tmp/root/.scafld/specs/drafts/fixture-task.md"}
}

func (s *memorySpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return s.model, s.path, nil
}

func (s *memorySpecStore) Save(_ context.Context, path string, model spec.Model) error {
	s.saves++
	if s.failSaveAt > 0 && s.saves == s.failSaveAt {
		return s.saveErr
	}
	s.path = path
	s.model = model
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
}

type fakeHardenProvider struct {
	dossier coreharden.Dossier
	err     error
}

func (p fakeHardenProvider) Invoke(_ context.Context, req coreharden.Request) (coreharden.Dossier, error) {
	if !strings.Contains(req.Prompt, "Harden Context Packet") && !strings.Contains(req.Prompt, "Review Context Packet") {
		return coreharden.Dossier{}, errors.New("missing harden context")
	}
	if p.err != nil {
		return coreharden.Dossier{}, p.err
	}
	return p.dossier, nil
}

func passingHardenDossier() coreharden.Dossier {
	observations := passingObservations()
	out := coreharden.Dossier{
		Summary:      "The draft is scoped and ready for approval.",
		Provider:     "codex",
		Model:        "gpt-test",
		OutputFormat: "codex.output_file",
		Observations: make([]coreharden.Observation, 0, len(observations)),
	}
	for _, observation := range observations {
		out.Observations = append(out.Observations, coreharden.Observation{
			Dimension: observation.Dimension,
			Result:    observation.Result,
			Anchor:    observation.Anchor,
			Note:      observation.Note,
			Default:   observation.Default,
			Status:    observation.Status,
		})
	}
	return out
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
