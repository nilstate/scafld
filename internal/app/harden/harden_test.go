package harden

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/agentcontract"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
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
	if !strings.Contains(out.Context, "## Source Spec Markdown") || !strings.Contains(out.Context, "# Fixture task") {
		t.Fatalf("context missing source markdown:\n%s", out.Context)
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
	if store.model.CurrentState.Blockers != manualHardenBlocker() || !strings.Contains(store.model.CurrentState.AllowedFollowUp, "fill harden observations in") || !strings.Contains(store.model.CurrentState.AllowedFollowUp, out.NextCommand) {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
	if store.model.HardenRounds[0].SpecDigest == "" || out.SpecDigest != store.model.HardenRounds[0].SpecDigest {
		t.Fatalf("spec digest not recorded: out=%q round=%q", out.SpecDigest, store.model.HardenRounds[0].SpecDigest)
	}
}

func TestRunReusesOpenManualHardenRound(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		StartedAt:    "2026-05-04T00:00:00Z",
		SpecDigest:   spec.HardenContractDigest(model),
		Observations: hardenObservationSkeleton(),
	}}
	store := newMemorySpecStore(model)
	store.sourceMarkdown = []byte("# Fixture task\n\n## Summary\n\n## Harden Rounds\n\n### round-1\n\nStatus: in_progress\n")
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", Prompt: "prompt body"})
	if err != nil {
		t.Fatal(err)
	}
	if out.RoundID != "round-1" || len(store.model.HardenRounds) != 1 || store.saves != 0 {
		t.Fatalf("rerun should reuse current round without saving: out=%+v rounds=%+v saves=%d", out, store.model.HardenRounds, store.saves)
	}
	if !strings.Contains(out.Context, "### round-1") {
		t.Fatalf("context did not render existing round:\n%s", out.Context)
	}
}

func TestRunProviderHardenBlocksWhenRoundOpen(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenInProgress),
		StartedAt:  "2026-05-04T00:00:00Z",
		SpecDigest: spec.HardenContractDigest(model),
	}}
	called := false
	store := newMemorySpecStore(model)
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	})
	if !errors.Is(err, ErrHardenRoundOpen) {
		t.Fatalf("error = %v, want %v", err, ErrHardenRoundOpen)
	}
	if called || len(store.model.HardenRounds) != 1 {
		t.Fatalf("provider should not run or append round: called=%v rounds=%+v", called, store.model.HardenRounds)
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
		{ID: "round-2", Status: string(spec.HardenInProgress), Shape: passingShape(), Observations: passingObservations()},
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
		Shape:        passingShape(),
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
	if !strings.Contains(gateErr.Failure.Actual, "harden evidence issue") || len(gateErr.Failure.Blockers) <= len(out.Warnings) {
		t.Fatalf("gate failure did not summarize warnings: %#v", gateErr.Failure)
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
		Shape:        passingShape(),
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
		Shape:  passingShape(),
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
	observations[0] = spec.HardenObservation{
		Dimension: "design",
		Result:    coreharden.ResultBlocks,
		Anchor:    "spec_gap:Summary",
		Note:      "Plan treats a symptom.",
		Status:    coreharden.StatusOpen,
	}
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Shape:        passingShape(),
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
	observations[5] = spec.HardenObservation{
		Dimension: "rollback",
		Result:    coreharden.ResultAdvisory,
		Anchor:    "spec_gap:Rollback",
		Note:      "Rollback could name a recovery command.",
	}
	model.HardenRounds = []spec.HardenRound{{
		ID:           "round-1",
		Status:       string(spec.HardenInProgress),
		Shape:        passingShape(),
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
	if round.Shape.Decision != coreharden.DecisionKeep || round.Shape.TrueShape == "" {
		t.Fatalf("shape not recorded: %+v", round.Shape)
	}
	if round.SpecDigest == "" || out.SpecDigest != round.SpecDigest {
		t.Fatalf("spec digest not recorded: out=%q round=%q", out.SpecDigest, round.SpecDigest)
	}
}

func TestRunProviderHardenNoopsWhenSameRevisionAlreadyPassed(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	if _, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier()},
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	out, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called || len(store.model.HardenRounds) != 1 {
		t.Fatalf("same-revision pass should not rerun provider: called=%v rounds=%+v", called, store.model.HardenRounds)
	}
	if out.HardenStatus != spec.HardenPassed || out.NextCommand != "scafld approve fixture-task" {
		t.Fatalf("output = %+v", out)
	}
}

func TestRunProviderHardenUsesLatestRoundOverStaleProjection(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenPassed),
		StartedAt:  "2026-05-04T00:00:00Z",
		EndedAt:    "2026-05-04T00:01:00Z",
		SpecDigest: spec.HardenContractDigest(model),
		Verdict:    coreharden.VerdictPass,
		Summary:    "already passed",
	}}
	called := false
	out, err := Run(context.Background(), newMemorySpecStore(model), fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	})
	if err != nil {
		t.Fatal(err)
	}
	if called || out.HardenStatus != spec.HardenPassed || out.NextCommand != "scafld approve fixture-task" {
		t.Fatalf("stale projection should no-op from latest round: called=%v out=%+v", called, out)
	}
}

func TestRunProviderHardenUsesProviderOutputContractOnly(t *testing.T) {
	t.Parallel()

	contract := testContract(t, agentcontract.RoleHarden, "# Harden Contract\n\nA clean result must say what was checked and what would have failed it.")
	var prompt string
	provider := fakeHardenProvider{
		dossier:       passingHardenDossier(),
		capturePrompt: &prompt,
	}
	store := newMemorySpecStore(fixtureModel())
	if _, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Contract: contract,
		Provider: provider,
	}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Harden Contract", "what was checked", "## Provider Harden Output Contract", "submit_harden", "## Provider Instruction"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("provider prompt missing %q:\n%s", want, prompt)
		}
	}
	if !manifestMarksRequired(prompt, "provider_instruction", "Provider Instruction") {
		t.Fatalf("provider instruction was not marked required:\n%s", prompt)
	}
	if strings.Contains(prompt, "Manual Harden Output Contract") || strings.Contains(prompt, "--mark-passed") {
		t.Fatalf("provider prompt includes manual output contract:\n%s", prompt)
	}
}

func manifestMarksRequired(prompt string, key string, title string) bool {
	needle := "`" + key + "` (" + title + ")"
	for _, line := range strings.Split(prompt, "\n") {
		if strings.Contains(line, needle) && strings.Contains(line, "required=true") {
			return true
		}
	}
	return false
}

func TestRunProviderHardenRecordsNeedsRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[0] = coreharden.Observation{
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
	if !strings.Contains(store.model.CurrentState.Blockers, "unresolved blocking observation") ||
		!strings.Contains(store.model.CurrentState.AllowedFollowUp, "operator decision") ||
		!strings.Contains(store.model.CurrentState.AllowedFollowUp, "--provider <provider>") ||
		!strings.Contains(store.model.CurrentState.AllowedFollowUp, "scafld approve fixture-task") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenBlocksSameRevisionAfterNeedsRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[0] = coreharden.Observation{
		Dimension: "design",
		Result:    coreharden.ResultBlocks,
		Anchor:    "spec_gap:Summary",
		Note:      "The plan may be future bloat.",
		Status:    coreharden.StatusOpen,
	}
	store := newMemorySpecStore(fixtureModel())
	if _, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	})
	if !errors.Is(err, ErrHardenRevisionRequired) {
		t.Fatalf("error = %v, want %v", err, ErrHardenRevisionRequired)
	}
	if called || len(store.model.HardenRounds) != 1 {
		t.Fatalf("same-revision rerun should not spend provider: called=%v rounds=%+v", called, store.model.HardenRounds)
	}
	var gateErr gate.Error
	if !errors.As(err, &gateErr) || !strings.Contains(gateErr.Failure.Next, "operator decision") || !strings.Contains(gateErr.Failure.Next, "scafld approve fixture-task") {
		t.Fatalf("gate failure should expose operator decision: %#v", gateErr.Failure)
	}
}

func TestRunProviderHardenAllowsRerunAfterDraftRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Shape.Decision = coreharden.DecisionReframe
	dossier.Shape.RequiredSpecEdits = []string{"Move the shared behavior to the core owner."}
	store := newMemorySpecStore(fixtureModel())
	if _, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: dossier},
	}); err != nil {
		t.Fatal(err)
	}
	revised := store.model
	revised.Summary = revised.Summary + " Revised to name the shared owner."
	store.model = revised
	called := false
	if _, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	}); err != nil {
		t.Fatal(err)
	}
	if !called || len(store.model.HardenRounds) != 2 {
		t.Fatalf("revision should allow a new provider round: called=%v rounds=%+v", called, store.model.HardenRounds)
	}
	if store.model.HardenRounds[0].SpecDigest == store.model.HardenRounds[1].SpecDigest {
		t.Fatalf("new round kept old digest: %+v", store.model.HardenRounds)
	}
}

func TestRunProviderHardenShapeDecisionRequiresRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Shape.Decision = coreharden.DecisionShrink
	dossier.Shape.RequiredSpecEdits = []string{"Remove adapter-specific duplication before approval."}
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
	if !strings.Contains(store.model.CurrentState.Blockers, "shape decision requires revision") || !strings.Contains(store.model.CurrentState.Blockers, "required spec edit") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenAdvisoryObservationsDoNotBlock(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Observations[5] = coreharden.Observation{
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

func TestRunProviderHardenClosesFailureAfterCallerContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	store := &contextCheckingSpecStore{memorySpecStore: newMemorySpecStore(fixtureModel())}
	_, err := Run(ctx, store, fixedClock{}, Input{
		TaskID: "fixture-task",
		Provider: hardenProviderFunc(func(context.Context, coreharden.Request) (coreharden.Dossier, error) {
			cancel()
			return coreharden.Dossier{}, context.Canceled
		}),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context canceled", err)
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || !strings.Contains(round.Summary, "context canceled") {
		t.Fatalf("round = %+v", round)
	}
}

func TestRunProviderHardenRecordsDossierAfterCallerContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	store := &contextCheckingSpecStore{memorySpecStore: newMemorySpecStore(fixtureModel())}
	out, err := Run(ctx, store, fixedClock{}, Input{
		TaskID: "fixture-task",
		Provider: hardenProviderFunc(func(context.Context, coreharden.Request) (coreharden.Dossier, error) {
			cancel()
			return passingHardenDossier(), nil
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != coreharden.VerdictPass || store.model.HardenStatus != spec.HardenPassed {
		t.Fatalf("harden output = %+v status=%s", out, store.model.HardenStatus)
	}
}

func TestRunProviderHardenTerminalReloadFailureClosesOriginalTaskRound(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	store.failLoadAt = 1
	store.loadErr = errors.New("reload failed")
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier()},
	})
	if err == nil || !strings.Contains(err.Error(), "load harden round after provider") {
		t.Fatalf("err = %v, want terminal reload failure", err)
	}
	if store.model.TaskID != "fixture-task" {
		t.Fatalf("recovery wrote wrong task: %+v", store.model)
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || !strings.Contains(round.Summary, "provider harden terminal recording failed") || !strings.Contains(round.Summary, "reload failed") {
		t.Fatalf("round = %+v", round)
	}
	if !strings.Contains(store.model.CurrentState.AllowedFollowUp, "fixture-task") {
		t.Fatalf("current state lost task id: %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenClosesRoundWhenContextCancelsAfterInitialSave(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	called := false
	store := &contextCheckingSpecStore{memorySpecStore: newMemorySpecStore(fixtureModel())}
	store.memorySpecStore.afterSave = func(saves int) {
		if saves == 1 {
			cancel()
		}
	}
	_, err := Run(ctx, store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier(), called: &called},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context canceled", err)
	}
	if called {
		t.Fatal("provider should not run after source reload cancellation")
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || !strings.Contains(round.Summary, "provider harden source reload failed") {
		t.Fatalf("round = %+v", round)
	}
	if store.model.CurrentState.Reason != "external hardening setup error" || !strings.Contains(store.model.CurrentState.AllowedFollowUp, "fix local spec access") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenCompactsProviderDiagnosticsInSpec(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	err := hardenDiagnosticErr{
		message: "provider failed: process idle timeout:\n" + strings.Repeat("WARN plugin loader noise\n", 40),
		path:    "/tmp/scafld-diagnostic.txt",
	}
	_, runErr := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{err: err},
	})
	if runErr == nil {
		t.Fatal("expected provider error")
	}
	round := store.model.HardenRounds[0]
	if strings.Contains(round.Summary, "\n") || len(round.Summary) > 340 {
		t.Fatalf("summary should be compact one-line text, got %d bytes:\n%s", len(round.Summary), round.Summary)
	}
	if round.DiagnosticPath != err.path {
		t.Fatalf("diagnostic path = %q, want %q", round.DiagnosticPath, err.path)
	}
	if !strings.Contains(round.Summary, "process idle timeout") || !strings.Contains(round.Summary, err.path) {
		t.Fatalf("summary lost cause or diagnostic path: %s", round.Summary)
	}
	if store.model.CurrentState.Blockers != round.Summary {
		t.Fatalf("current blockers should use compact summary: %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenRejectsOversizedRequiredContextBeforeProvider(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	store.sourceMarkdown = []byte("abcdef")
	providerCalled := false
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:                  "fixture-task",
		Provider:                fakeHardenProvider{dossier: passingHardenDossier(), called: &providerCalled},
		ContextMaxBytes:         3,
		RequiredContextMaxBytes: 3,
	})
	if !errors.Is(err, reviewcontext.ErrRequiredContextTooLarge) {
		t.Fatalf("error = %v, want %v", err, reviewcontext.ErrRequiredContextTooLarge)
	}
	if providerCalled {
		t.Fatal("provider was invoked despite oversized required context")
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || round.EndedAt == "" || !strings.Contains(round.Summary, "provider harden context invalid") {
		t.Fatalf("round = %+v", round)
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

func TestRunProviderHardenTerminalSaveFailureClosesRound(t *testing.T) {
	t.Parallel()

	store := newMemorySpecStore(fixtureModel())
	store.failSaveAt = 2
	store.saveErr = errors.New("terminal save failed")
	_, err := Run(context.Background(), store, fixedClock{}, Input{
		TaskID:   "fixture-task",
		Provider: fakeHardenProvider{dossier: passingHardenDossier()},
	})
	if err == nil || !strings.Contains(err.Error(), "save harden dossier") || !strings.Contains(err.Error(), "terminal save failed") {
		t.Fatalf("error = %v, want terminal save failure", err)
	}
	if store.model.HardenStatus != spec.HardenError {
		t.Fatalf("harden status = %s", store.model.HardenStatus)
	}
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenError) || round.EndedAt == "" || !strings.Contains(round.Summary, "provider harden terminal recording failed") {
		t.Fatalf("round = %+v", round)
	}
	if store.model.CurrentState.Reason != "external hardening terminal recording error" || !strings.Contains(store.model.CurrentState.AllowedFollowUp, "fix local spec storage") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func passingObservations() []spec.HardenObservation {
	return []spec.HardenObservation{
		{Dimension: "design", Result: coreharden.ResultClean, Anchor: "spec_gap:Summary", Note: "Shared owner and adapter boundaries are checked."},
		{Dimension: "scope", Result: coreharden.ResultClean, Anchor: "spec_gap:Risks", Note: "Migration claims checked."},
		{Dimension: "path", Result: coreharden.ResultClean, Anchor: "spec_gap:Scope", Note: "Paths checked."},
		{Dimension: "command", Result: coreharden.ResultClean, Anchor: "spec_gap:Validation", Note: "Commands checked."},
		{Dimension: "timing", Result: coreharden.ResultClean, Anchor: "spec_gap:Phases", Note: "Criteria timing checked."},
		{Dimension: "rollback", Result: coreharden.ResultNotApplicable, Anchor: "spec_gap:Rollback", Note: "Docs-only change has no runtime rollback."},
	}
}

func passingShape() spec.HardenShape {
	return spec.HardenShape{
		Decision:          coreharden.DecisionKeep,
		TrueShape:         "Shared context renderer with thin adapters.",
		MinimalPlan:       "Render raw Markdown first and keep derived sections secondary.",
		SharedOwner:       "internal/core/reviewcontext",
		AdapterBoundaries: []string{"harden and review build provider packets", "CLI surfaces print context by default"},
	}
}

type memorySpecStore struct {
	model          spec.Model
	path           string
	sourceMarkdown []byte
	saves          int
	loads          int

	failSaveAt int
	saveErr    error
	failLoadAt int
	loadErr    error
	afterSave  func(int)
}

func newMemorySpecStore(model spec.Model) *memorySpecStore {
	return &memorySpecStore{model: model, path: "/tmp/root/.scafld/specs/drafts/fixture-task.md"}
}

func (s *memorySpecStore) Load(context.Context, string) (spec.Model, string, error) {
	s.loads++
	if s.failLoadAt > 0 && s.loads == s.failLoadAt {
		return spec.Model{}, "", s.loadErr
	}
	return s.model, s.path, nil
}

func (s *memorySpecStore) LoadSource(context.Context, string) (spec.Source, error) {
	markdown := s.sourceMarkdown
	if markdown == nil {
		markdown = []byte("# " + s.model.Title + "\n\n## Summary\n\n" + s.model.Summary + "\n")
	}
	return spec.Source{Model: s.model, Path: s.path, Markdown: markdown}, nil
}

func (s *memorySpecStore) Save(_ context.Context, path string, model spec.Model) error {
	s.saves++
	if s.failSaveAt > 0 && s.saves == s.failSaveAt {
		return s.saveErr
	}
	s.path = path
	s.model = model
	if s.afterSave != nil {
		s.afterSave(s.saves)
	}
	return nil
}

type contextCheckingSpecStore struct {
	*memorySpecStore
}

func (s *contextCheckingSpecStore) Load(ctx context.Context, taskID string) (spec.Model, string, error) {
	if err := ctx.Err(); err != nil {
		return spec.Model{}, "", err
	}
	return s.memorySpecStore.Load(ctx, taskID)
}

func (s *contextCheckingSpecStore) LoadSource(ctx context.Context, taskID string) (spec.Source, error) {
	if err := ctx.Err(); err != nil {
		return spec.Source{}, err
	}
	return s.memorySpecStore.LoadSource(ctx, taskID)
}

func (s *contextCheckingSpecStore) Save(ctx context.Context, path string, model spec.Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.memorySpecStore.Save(ctx, path, model)
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
}

type fakeHardenProvider struct {
	dossier       coreharden.Dossier
	err           error
	called        *bool
	capturePrompt *string
}

type hardenProviderFunc func(context.Context, coreharden.Request) (coreharden.Dossier, error)

func (f hardenProviderFunc) Invoke(ctx context.Context, req coreharden.Request) (coreharden.Dossier, error) {
	return f(ctx, req)
}

type hardenDiagnosticErr struct {
	message string
	path    string
}

func (e hardenDiagnosticErr) Error() string { return e.message }

func (e hardenDiagnosticErr) DiagnosticPath() string { return e.path }

func (p fakeHardenProvider) Invoke(_ context.Context, req coreharden.Request) (coreharden.Dossier, error) {
	if p.called != nil {
		*p.called = true
	}
	if p.capturePrompt != nil {
		*p.capturePrompt = req.Prompt
	}
	if !strings.Contains(req.Prompt, "Harden Context Packet") && !strings.Contains(req.Prompt, "Review Context Packet") {
		return coreharden.Dossier{}, errors.New("missing harden context")
	}
	if !strings.Contains(req.Prompt, "## Source Spec Markdown") {
		return coreharden.Dossier{}, errors.New("missing source spec markdown")
	}
	if p.err != nil {
		return coreharden.Dossier{}, p.err
	}
	return p.dossier, nil
}

func testContract(t *testing.T, role agentcontract.Role, body string) agentcontract.Contract {
	t.Helper()
	contract, err := agentcontract.New(role, ".scafld/core/prompts/"+role.Filename(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func passingHardenDossier() coreharden.Dossier {
	observations := passingObservations()
	out := coreharden.Dossier{
		Summary: "The draft is scoped and ready for approval.",
		Shape: coreharden.Shape{
			Decision:          coreharden.DecisionKeep,
			TrueShape:         "Shared context renderer with thin adapters.",
			MinimalPlan:       "Render raw Markdown first and keep derived sections secondary.",
			SharedOwner:       "internal/core/reviewcontext",
			AdapterBoundaries: []string{"harden and review build provider packets", "CLI surfaces print context by default"},
		},
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
