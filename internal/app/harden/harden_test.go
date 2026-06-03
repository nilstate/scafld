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
	got := store.model
	if got.HardenStatus != spec.HardenInProgress {
		t.Fatalf("harden status = %s", got.HardenStatus)
	}
	if len(got.HardenRounds) != 1 || got.HardenRounds[0].Status != string(spec.HardenInProgress) || got.HardenRounds[0].StartedAt == "" {
		t.Fatalf("harden rounds = %+v", got.HardenRounds)
	}
	if len(got.HardenRounds[0].Checks) != len(requiredHardenChecks) {
		t.Fatalf("harden check skeleton = %+v", got.HardenRounds[0].Checks)
	}
	for i, check := range got.HardenRounds[0].Checks {
		if normalizeCheckName(check.Name) != requiredHardenChecks[i] || check.GroundedIn != "" || check.Result != "" || check.Evidence != "" {
			t.Fatalf("check skeleton[%d] = %+v", i, check)
		}
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
		{ID: "round-1", Status: string(spec.HardenNeedsRevision)},
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

func TestRunMarkPassedRejectsUngroundedIssues(t *testing.T) {
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
		Issues: []spec.HardenIssue{
			{ID: "harden-1", Kind: "question", Severity: "low", Status: "open", GroundedIn: "code:code.go:1", Summary: "Who owns this?", Evidence: "Owner not named.", Recommendation: "Use the owner.", Question: "Who owns this?", RecommendedAnswer: "Use the owner."},
			{ID: "harden-2", Kind: "question", Severity: "low", Status: "open", GroundedIn: "code:missing.go:1", Summary: "What proves this?", Evidence: "Missing file cited.", Recommendation: "Use missing.", Question: "What proves this?", RecommendedAnswer: "Use missing."},
			{ID: "harden-3", Kind: "question", Severity: "low", Status: "open", Summary: "Why now?", Evidence: "Missing grounding.", Recommendation: "Keep scope.", Question: "Why now?", RecommendedAnswer: "Because scope requires it."},
			{ID: "harden-4", Kind: "question", Severity: "low", Status: "open", GroundedIn: "code:code.go", Summary: "Where?", Evidence: "Invalid code citation.", Recommendation: "Use code.go.", Question: "Where?", RecommendedAnswer: "Use code.go."},
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

func TestRunMarkPassedRejectsIssueQuestionWithoutRecommendedAnswer(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: passedChecks(),
		Issues: []spec.HardenIssue{{
			ID:             "harden-1",
			Kind:           "question",
			Severity:       "low",
			BlocksApproval: false,
			Status:         "open",
			GroundedIn:     "spec_gap:Scope",
			Summary:        "Owner is not named.",
			Evidence:       "Scope omits the owner.",
			Recommendation: "Name the owner.",
			Question:       "Who owns this?",
		}},
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	joined := strings.Join(out.Warnings, "\n")
	if !strings.Contains(joined, "question issue missing recommended answer") {
		t.Fatalf("warnings did not reject incomplete question issue: %+v", out.Warnings)
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
	if round.Status != string(spec.HardenPassed) || round.Provider != "codex" || round.Model != "gpt-test" || round.Summary == "" || len(round.Checks) != 6 {
		t.Fatalf("round = %+v", round)
	}
}

func TestRunProviderHardenRecordsNeedsRevision(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Verdict = coreharden.VerdictNeedsRevision
	dossier.Checks[5].Result = "failed"
	dossier.Issues = []coreharden.Issue{{
		ID:                "harden-1",
		Kind:              "design_challenge",
		Severity:          "high",
		BlocksApproval:    true,
		Status:            "open",
		GroundedIn:        "spec_gap:Summary",
		Summary:           "The plan may be future bloat.",
		Evidence:          "The spec does not cite repeated use.",
		Recommendation:    "Reduce scope or cite the repeated need.",
		Question:          "Why add this abstraction?",
		RecommendedAnswer: "Do not add it until duplicated complexity is proven.",
	}}
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
	if round.Status != string(spec.HardenNeedsRevision) || len(round.Issues) != 1 {
		t.Fatalf("round = %+v", round)
	}
	if !strings.Contains(store.model.CurrentState.Blockers, "check needs revision") || !strings.Contains(store.model.CurrentState.Blockers, "approval-blocking issue") || !strings.Contains(store.model.CurrentState.AllowedFollowUp, "--provider <provider>") {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunProviderHardenAdvisoryIssuesDoNotBlock(t *testing.T) {
	t.Parallel()

	dossier := passingHardenDossier()
	dossier.Issues = []coreharden.Issue{{
		ID:                "harden-1",
		Kind:              "question",
		Severity:          "low",
		BlocksApproval:    false,
		Status:            "open",
		GroundedIn:        "spec_gap:Rollback",
		Summary:           "The rollback section could name a more convenient recovery command.",
		Evidence:          "Rollback is present, but it is generic.",
		Recommendation:    "Name the command if it is already known.",
		Question:          "What command should a human run if the change fails halfway?",
		RecommendedAnswer: "Use the package's existing recovery command if one exists.",
	}}
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
	round := store.model.HardenRounds[0]
	if round.Status != string(spec.HardenPassed) || len(round.Issues) != 1 || round.Issues[0].BlocksApproval {
		t.Fatalf("round = %+v", round)
	}
	if store.model.CurrentState.AllowedFollowUp != "scafld approve fixture-task" {
		t.Fatalf("current state = %+v", store.model.CurrentState)
	}
}

func TestRunMarkPassedRejectsOpenApprovalBlockingIssue(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: passedChecks(),
		Issues: []spec.HardenIssue{{
			ID:             "harden-1",
			Kind:           "design_challenge",
			Severity:       "high",
			BlocksApproval: true,
			Status:         "open",
			GroundedIn:     "spec_gap:Summary",
			Summary:        "Plan is a bandaid.",
			Evidence:       "Summary does not name the root cause.",
			Recommendation: "Fix the root cause or narrow the plan.",
		}},
	}}
	store := newMemorySpecStore(model)
	out, err := Run(context.Background(), store, fixedClock{}, Input{TaskID: "fixture-task", MarkPassed: true})
	if !errors.Is(err, ErrInvalidHardenEvidence) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidHardenEvidence)
	}
	if !strings.Contains(strings.Join(out.Warnings, "\n"), "approval-blocking issue is still open") {
		t.Fatalf("warnings = %+v", out.Warnings)
	}
}

func TestRunMarkPassedAllowsOpenAdvisoryIssue(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenInProgress),
		Checks: passedChecks(),
		Issues: []spec.HardenIssue{{
			ID:             "harden-1",
			Kind:           "recommended_edit",
			Severity:       "low",
			BlocksApproval: false,
			Status:         "open",
			GroundedIn:     "spec_gap:Rollback",
			Summary:        "Rollback could be clearer.",
			Evidence:       "Rollback exists but is terse.",
			Recommendation: "Expand rollback if convenient.",
		}},
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
		Provider: fakeHardenProvider{dossier: coreharden.Dossier{Verdict: coreharden.VerdictPass}},
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
	checks := passedChecks()
	out := coreharden.Dossier{
		Verdict:      coreharden.VerdictPass,
		Summary:      "The draft is scoped and ready for approval.",
		Provider:     "codex",
		Model:        "gpt-test",
		OutputFormat: "codex.output_file",
		Checks:       make([]coreharden.Check, 0, len(checks)),
		AttackLog:    []coreharden.AttackLogEntry{{Target: "draft", Attack: "challenge core design", Result: "clean"}},
	}
	for _, check := range checks {
		out.Checks = append(out.Checks, coreharden.Check{
			Name:       strings.ToLower(check.Name),
			GroundedIn: check.GroundedIn,
			Result:     check.Result,
			Evidence:   check.Evidence,
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
