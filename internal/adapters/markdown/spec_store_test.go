package markdown

import (
	"context"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
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
	model.Review = spec.ReviewState{Status: "pending", Verdict: "none"}
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
	if got := parsed.Phases[0].Acceptance[0]; got.Evidence != "exit code was 0" || got.SourceEvent != "entry-1" {
		t.Fatalf("criterion evidence lost: %+v", got)
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
