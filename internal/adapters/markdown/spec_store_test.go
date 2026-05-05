package markdown

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestGoldenRoundTripReleasedExamples(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	rendered := Render(model)
	parsed, err := Parse(rendered)
	if err != nil {
		t.Fatal(err)
	}
	again := Render(parsed)
	if string(rendered) != string(again) {
		t.Fatalf("render is not byte-stable\nfirst:\n%s\nsecond:\n%s", rendered, again)
	}
}

func TestRoundTripPreservesLiterateSpecFields(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.CurrentState = spec.CurrentState{
		CurrentPhase:       "phase1",
		Next:               "review",
		Reason:             "runner update",
		Blockers:           "none",
		AllowedFollowUp:    "scafld review fixture-task",
		LatestRunnerUpdate: "2026-05-04T00:00:00Z",
		ReviewGate:         "not_started",
	}
	model.Objectives = []string{"Keep specs readable", "Keep execution evidence deterministic"}
	model.Scope = []string{"Markdown parser", "Renderer"}
	model.Dependencies = []string{"go toolchain"}
	model.Assumptions = []string{"No legacy YAML task specs"}
	model.Touchpoints = []string{"CLI", "agent workflow"}
	model.Rollback = []string{"Revert the parser change"}
	model.SelfEval = []string{"Round-trip checked"}
	model.Deviations = []string{"none observed"}
	model.Metadata = map[string]string{"owner": "runtime"}
	model.Origin = spec.Origin{CreatedBy: "test", Source: "golden"}
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:        "round-1",
		Status:    string(spec.HardenInProgress),
		StartedAt: "2026-05-04T00:00:00Z",
		Questions: []spec.HardenQuestion{{
			Question:          "What owns the parser contract?",
			GroundedIn:        "code:internal/adapters/markdown/spec_store.go:1",
			RecommendedAnswer: "The markdown adapter owns the grammar.",
			IfUnanswered:      "Keep grammar changes in the adapter.",
			AnsweredWith:      "Adapter-owned.",
		}},
	}}
	model.Review = spec.ReviewState{Status: "completed", Verdict: corereview.VerdictFail, Findings: []corereview.Finding{{ID: "f1", Severity: corereview.SeverityBlocking, Summary: "bug"}}}
	model.Phases[0].Dependencies = []string{"phase0"}
	model.Phases[0].Acceptance[0].Status = "pass"
	model.Phases[0].Acceptance[0].Evidence = "exit code was 0"
	model.Phases[0].Acceptance[0].SourceEvent = "entry-1"

	parsed, err := Parse(Render(model))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CurrentState.AllowedFollowUp != model.CurrentState.AllowedFollowUp {
		t.Fatalf("current state lost: %+v", parsed.CurrentState)
	}
	if len(parsed.Objectives) != 2 || parsed.Objectives[0] != "Keep specs readable" {
		t.Fatalf("objectives lost: %+v", parsed.Objectives)
	}
	if got := parsed.Phases[0].Dependencies; len(got) != 1 || got[0] != "phase0" {
		t.Fatalf("phase dependencies lost: %+v", got)
	}
	if parsed.Metadata["owner"] != "runtime" || parsed.Origin.CreatedBy != "test" {
		t.Fatalf("metadata/origin lost: %+v %+v", parsed.Metadata, parsed.Origin)
	}
	if parsed.HardenStatus != spec.HardenInProgress || len(parsed.HardenRounds) != 1 || parsed.HardenRounds[0].Status != string(spec.HardenInProgress) {
		t.Fatalf("harden state lost: %s %+v", parsed.HardenStatus, parsed.HardenRounds)
	}
	if got := parsed.HardenRounds[0].Questions[0]; got.GroundedIn == "" || got.AnsweredWith != "Adapter-owned." {
		t.Fatalf("harden question lost: %+v", got)
	}
	if got := parsed.Phases[0].Acceptance[0]; got.Evidence != "exit code was 0" || got.SourceEvent != "entry-1" {
		t.Fatalf("criterion evidence lost: %+v", got)
	}
	if got := parsed.Review.Findings; len(got) != 1 || got[0].Summary != "bug" {
		t.Fatalf("review findings lost: %+v", got)
	}
}

func TestRejectMalformedFrontMatterDuplicatePhaseUnclosedFenceAndMismatch(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"malformed front matter": "# Missing\n",
		"duplicate phases":       strings.ReplaceAll(string(Render(fixtureModel())), "## Phase 1: Implementation", "## Phase 1: Implementation\n\n## Phase 1: Duplicate"),
		"unclosed fence":         "---\nspec_version: '2.0'\ntask_id: bad\nstatus: draft\n---\n\n```go\n## Phase 1: hidden\n",
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(input)); err == nil {
				t.Fatal("expected parse rejection")
			}
		})
	}
}

func TestUpdateSpecMarkdownIgnoresHeadingLikeTextInsideCodeFences(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel())) + "\n```text\n## Phase 1: Not a phase\n```\n"
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Phases) != 1 {
		t.Fatalf("phase count = %d, want 1", len(parsed.Phases))
	}
}

func TestSavePreservesUnparsedSectionsWhenUpdatingHardenState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	path, err := store.CreateDraft(context.Background(), fixtureModel())
	if err != nil {
		t.Fatal(err)
	}
	original := string(Render(fixtureModel()))
	original = strings.Replace(original, "## Objectives\n\n", "## Context\n\nFiles impacted:\n- `src/app.ts` - keep this human-owned detail\n\n## Objectives\n\n", 1)
	original = strings.Replace(original, "## Risks\n\n- none\n", "## Risks\n\n- Data loss during harden - preserve sections the parser does not own\n  - Mitigation: targeted section writes\n", 1)
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	model, loadedPath, err := store.Load(context.Background(), "fixture-task")
	if err != nil {
		t.Fatal(err)
	}
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = append(model.HardenRounds, spec.HardenRound{
		ID:        "round-1",
		Status:    string(spec.HardenInProgress),
		StartedAt: "2026-05-04T00:00:00Z",
	})
	model.CurrentState.Next = "harden"
	model.CurrentState.AllowedFollowUp = "scafld harden fixture-task --mark-passed"
	if err := store.Save(context.Background(), loadedPath, model); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, want := range []string{
		"Files impacted:\n- `src/app.ts` - keep this human-owned detail",
		"Data loss during harden - preserve sections the parser does not own",
		"harden_status: in_progress",
		"## Harden Rounds\n\n### round-1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved spec lost %q:\n%s", want, text)
		}
	}
}

func TestSpecStoreCreateLoadListAndValidate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	path, err := store.CreateDraft(context.Background(), fixtureModel())
	if err != nil {
		t.Fatal(err)
	}
	loaded, loadedPath, err := store.Load(context.Background(), "fixture-task")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.TaskID != "fixture-task" || loadedPath != path {
		t.Fatalf("loaded %s at %s", loaded.TaskID, loadedPath)
	}
	records, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
}

func TestCreateDraftDoesNotOverwriteExistingSpec(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	if _, err := store.CreateDraft(context.Background(), fixtureModel()); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDraft(context.Background(), fixtureModel()); !errors.Is(err, ErrSpecExists) {
		t.Fatalf("CreateDraft error = %v, want %v", err, ErrSpecExists)
	}
}

func TestSaveMovesSpecToLifecycleDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	model := fixtureModel()
	path, err := store.CreateDraft(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}

	model.Status = spec.StatusApproved
	model.Updated = "2026-05-05T00:00:00Z"
	if err := store.Save(context.Background(), path, model); err != nil {
		t.Fatal(err)
	}
	approved := filepath.Join(root, ".scafld", "specs", "approved", "fixture-task.md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("draft path still exists: %v", err)
	}
	if _, err := os.Stat(approved); err != nil {
		t.Fatalf("approved path missing: %v", err)
	}

	model.Status = spec.StatusCompleted
	model.Updated = "2026-05-05T01:00:00Z"
	if err := store.Save(context.Background(), approved, model); err != nil {
		t.Fatal(err)
	}
	archived := filepath.Join(root, ".scafld", "specs", "archive", "2026-05", "fixture-task.md")
	if _, err := os.Stat(approved); !os.IsNotExist(err) {
		t.Fatalf("approved path still exists after archive: %v", err)
	}
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archive path missing: %v", err)
	}
}

func FuzzParse(f *testing.F) {
	f.Add(string(Render(fixtureModel())))
	f.Add("---\nspec_version: '2.0'\ntask_id: fuzz\nstatus: draft\n---\n\n# Fuzz\n\n## Summary\n\ntext\n")
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = Parse([]byte(input))
	})
}

func fixtureModel() spec.Model {
	return spec.Model{
		Version:      "2.0",
		TaskID:       "fixture-task",
		Created:      "2026-05-01T00:00:00Z",
		Updated:      "2026-05-01T00:00:00Z",
		Title:        "Fixture task",
		Summary:      "A readable Markdown spec.",
		Status:       spec.StatusDraft,
		HardenStatus: spec.HardenNotRun,
		Size:         spec.SizeSmall,
		RiskLevel:    spec.RiskLow,
		Acceptance:   spec.Acceptance{ValidationProfile: "standard"},
		Phases: []spec.Phase{{
			ID:        "phase1",
			Number:    1,
			Name:      "Implementation",
			Status:    "pending",
			Objective: "Build the slice.",
			Changes:   []string{"Add tests."},
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				Type:         "command",
				Title:        "runs",
				PhaseID:      "phase1",
				Command:      "true",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pending",
			}},
		}},
		Metadata: map[string]string{},
	}
}
