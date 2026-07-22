package markdown

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	model.Context = spec.Context{
		CWD:           "/repo",
		Packages:      []string{"internal/core/reviewscope"},
		FilesImpacted: []string{"`internal/core/reviewscope/scope.go` - derive task scope from explicit files"},
		Invariants:    []string{"Context markdown is source-of-truth input."},
		RelatedDocs:   []string{"`docs/review.md`"},
	}
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
		ID:             "round-1",
		Status:         string(spec.HardenInProgress),
		StartedAt:      "2026-05-04T00:00:00Z",
		SpecDigest:     "abc123",
		DiagnosticPath: "/tmp/scafld-harden-diagnostic.txt",
		Shape: spec.HardenShape{
			Decision:          "keep",
			TrueShape:         "Markdown-first context gate.",
			MinimalPlan:       "Add source context and keep adapters thin.",
			SharedOwner:       "internal/core/reviewcontext",
			AdapterBoundaries: []string{"CLI renders context", "providers consume context"},
		},
		Observations: []spec.HardenObservation{{
			Dimension: "path",
			Result:    "clean",
			Anchor:    "spec_gap:Scope",
			Note:      "Scope paths checked.",
		}, {
			Dimension:    "rollback",
			Result:       "advisory",
			Anchor:       "spec_gap:Rollback",
			Note:         "Rollback could name a recovery command.",
			Question:     "What recovery command should be used?",
			Recommended:  "Use the existing repair command.",
			IfUnanswered: "Treat rollback as advisory only.",
			Default:      "Use the existing repair command.",
		}},
	}}
	model.Review = spec.ReviewState{
		Status:         "completed",
		Verdict:        corereview.VerdictFail,
		Mode:           corereview.ModeDiscover,
		Provider:       "claude",
		Model:          "claude-test",
		OutputFormat:   "claude.mcp_submit_review",
		Normalizations: []string{"normalized one"},
		Summary:        "Review found an open blocker.",
		Findings: []corereview.Finding{{
			ID:               "f1",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Category:         "schema",
			Confidence:       corereview.ConfidenceHigh,
			Location:         &corereview.Location{Path: "file.go"},
			Evidence:         "bug",
			Impact:           "test impact",
			Reproducer:       "render then parse",
			SuggestedFix:     "preserve fields",
			Validation:       "rerun test",
			RelatedSpec:      "Review",
			ReviewPass:       "verify",
			Status:           corereview.FindingOpen,
			Summary:          "bug",
		}},
	}
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
	if parsed.Context.CWD != model.Context.CWD || !reflect.DeepEqual(parsed.Context.Packages, model.Context.Packages) || !reflect.DeepEqual(parsed.Context.FilesImpacted, model.Context.FilesImpacted) || !reflect.DeepEqual(parsed.Context.Invariants, model.Context.Invariants) || !reflect.DeepEqual(parsed.Context.RelatedDocs, model.Context.RelatedDocs) {
		t.Fatalf("context lost: %+v", parsed.Context)
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
	if got := parsed.HardenRounds[0].Observations[0]; got.Dimension != "path" || got.Result != "clean" || got.Anchor != "spec_gap:Scope" {
		t.Fatalf("harden observation lost: %+v", got)
	}
	if got := parsed.HardenRounds[0].Observations[1]; got.Dimension != "rollback" || got.Result != "advisory" || got.Default == "" || got.Question == "" || got.Recommended == "" || got.IfUnanswered == "" {
		t.Fatalf("harden advisory observation lost: %+v", got)
	}
	if parsed.HardenRounds[0].DiagnosticPath != "/tmp/scafld-harden-diagnostic.txt" {
		t.Fatalf("harden diagnostic path lost: %+v", parsed.HardenRounds[0])
	}
	if parsed.HardenRounds[0].SpecDigest != "abc123" {
		t.Fatalf("harden spec digest lost: %+v", parsed.HardenRounds[0])
	}
	if got := parsed.HardenRounds[0].Shape; got.Decision != "keep" || got.SharedOwner != "internal/core/reviewcontext" || len(got.AdapterBoundaries) != 2 {
		t.Fatalf("harden shape lost: %+v", got)
	}
	if got := parsed.Phases[0].Acceptance[0]; got.Evidence != "exit code was 0" || got.SourceEvent != "entry-1" {
		t.Fatalf("criterion evidence lost: %+v", got)
	}
	if got := parsed.Review.Findings; len(got) != 1 || got[0].Summary != "bug" {
		t.Fatalf("review findings lost: %+v", got)
	}
	if got := parsed.Review.Findings[0]; got.Category != "schema" || got.Confidence != corereview.ConfidenceHigh || got.Reproducer != "render then parse" || got.SuggestedFix != "preserve fields" || got.RelatedSpec != "Review" || got.ReviewPass != "verify" || got.Status != corereview.FindingOpen {
		t.Fatalf("review finding details lost: %+v", got)
	}
	if parsed.Review.Provider != "claude" || parsed.Review.Model != "claude-test" || parsed.Review.OutputFormat != "claude.mcp_submit_review" || len(parsed.Review.Normalizations) != 1 || parsed.Review.Normalizations[0] != "normalized one" {
		t.Fatalf("review provenance lost: %+v", parsed.Review)
	}
}

func TestParsePreservesWrappedListItems(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.Objectives = []string{"Keep specs readable"}
	model.Touchpoints = []string{"CLI"}
	input := string(Render(model))
	input = strings.Replace(input, "## Objectives\n\n- Keep specs readable", "## Objectives\n\n- Keep specs readable\n  across wrapped lines", 1)
	input = strings.Replace(input, "## Touchpoints\n\n- CLI", "## Touchpoints\n\n- `docs/review.md`, `docs/sourcey.config.ts`: docs nav\n  and review guidance", 1)
	input = strings.Replace(input, "Changes:\n- Add tests.", "Changes:\n- Add tests.\n  And update renderer.", 1)
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Objectives[0]; got != "Keep specs readable across wrapped lines" {
		t.Fatalf("wrapped objective = %q", got)
	}
	if got := parsed.Touchpoints[0]; got != "`docs/review.md`, `docs/sourcey.config.ts`: docs nav and review guidance" {
		t.Fatalf("wrapped touchpoint = %q", got)
	}
	if got := parsed.Phases[0].Changes[0]; got != "Add tests. And update renderer." {
		t.Fatalf("wrapped phase change = %q", got)
	}
}

func TestParseOnlyReadsCriteriaFromAcceptanceBlocks(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	input = strings.Replace(input, "## Objectives\n\n- none", "## Objectives\n\n- [ ] `not-ac` command - This is prose, not a criterion.", 1)
	input = strings.Replace(input, "Changes:\n- Add tests.", "Changes:\n- [ ] `not-phase-ac` command - This is a change bullet, not acceptance.", 1)
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Acceptance.Criteria) != 0 {
		t.Fatalf("global criteria polluted from non-acceptance sections: %+v", parsed.Acceptance.Criteria)
	}
	if len(parsed.Phases) != 1 || len(parsed.Phases[0].Acceptance) != 1 || parsed.Phases[0].Acceptance[0].ID != "ac1" {
		t.Fatalf("phase acceptance = %+v", parsed.Phases)
	}
	if len(parsed.Objectives) != 1 || !strings.Contains(parsed.Objectives[0], "not-ac") {
		t.Fatalf("objective bullet lost: %+v", parsed.Objectives)
	}
	if len(parsed.Phases[0].Changes) != 1 || !strings.Contains(parsed.Phases[0].Changes[0], "not-phase-ac") {
		t.Fatalf("phase change bullet lost: %+v", parsed.Phases[0].Changes)
	}
}

func TestRenderOmitsEmptyTopLevelValidationBlock(t *testing.T) {
	t.Parallel()

	rendered := string(Render(fixtureModel()))
	if strings.Contains(rendered, "## Acceptance\n\nProfile: standard\n\nValidation:\n- none") {
		t.Fatalf("empty top-level validation block should not render:\n%s", rendered)
	}
	if !strings.Contains(rendered, "## Acceptance\n\nProfile: standard\n\n## Phase 1") {
		t.Fatalf("top-level acceptance should only render profile when no global criteria:\n%s", rendered)
	}
}

func TestParseBareAcceptanceCriterionUsesCommandDetails(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	input = strings.Replace(input, "- [ ] `ac1` command - runs", "- [ ] `ac1` runs", 1)
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	got := parsed.Phases[0].Acceptance[0]
	if got.ID != "ac1" || got.Type != "command" || got.Title != "runs" || got.Command != "true" || got.ExpectedKind != acceptance.ExpectedExitCodeZero {
		t.Fatalf("bare criterion parsed incorrectly: %+v", got)
	}
}

func TestParseDefaultsBrowserCriteriaToBrowserEvidence(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	input = strings.Replace(input, "- [ ] `ac1` command - runs", "- [ ] `ac1` browser - Browser smoke passes", 1)
	input = strings.Replace(input, "  - Expected kind: `exit_code_zero`\n", "", 1)
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	got := parsed.Phases[0].Acceptance[0]
	if got.Type != "browser" || got.ExpectedKind != acceptance.ExpectedBrowserEvidence {
		t.Fatalf("browser criterion = %+v", got)
	}
}

func TestParseDefaultsManualCriteriaToManualEvidence(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	input = strings.Replace(input, "- [ ] `ac1` command - runs", "- [ ] `ac1` manual - Operator signoff", 1)
	input = strings.Replace(input, "  - Command: `true`\n", "", 1)
	input = strings.Replace(input, "  - Expected kind: `exit_code_zero`\n", "", 1)
	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	got := parsed.Phases[0].Acceptance[0]
	if got.Type != "manual" || got.ExpectedKind != acceptance.ExpectedManual || got.Command != "" {
		t.Fatalf("manual criterion = %+v", got)
	}
}

func TestRenderDefaultsManualCriteriaToManualEvidence(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.Phases[0].Acceptance[0] = spec.Criterion{
		ID:      "signoff",
		Type:    "manual",
		Title:   "Operator signoff",
		PhaseID: "phase1",
		Status:  "pending",
	}
	rendered := string(Render(model))
	if !strings.Contains(rendered, "- [ ] `signoff` manual - Operator signoff\n  - Expected kind: `manual`\n") {
		t.Fatalf("manual criterion did not render expected kind manual:\n%s", rendered)
	}
}

func TestParseRoundTripsHardenObservations(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	harden := `## Harden Rounds

### round-1

Status: in_progress
Started: 2026-05-04T00:00:00Z
Ended: none

Observations:
- design
  - Result: blocks
  - Anchor: spec_gap:Summary
  - Note: The plan is a bandaid.
  - Default: Fix the root cause or narrow the plan.
  - Status: open
- rollback
  - Result: advisory
  - Anchor: spec_gap:Rollback
  - Note: The rollback could name a recovery command.
  - Default: Use the existing repair command.

## Planning Log`
	input = strings.Replace(input, "## Harden Rounds\n\n- none\n\n## Planning Log", harden, 1)

	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	observations := parsed.HardenRounds[0].Observations
	if len(observations) != 2 {
		t.Fatalf("observations = %+v", observations)
	}
	if observations[0].Dimension != "design" || observations[0].Result != "blocks" || observations[0].Status != "open" {
		t.Fatalf("blocking observation = %+v", observations[0])
	}
	if observations[1].Dimension != "rollback" || observations[1].Result != "advisory" || observations[1].Default == "" {
		t.Fatalf("advisory observation = %+v", observations[1])
	}
	rendered := string(Render(parsed))
	if !strings.Contains(rendered, "Observations:\n- design") ||
		!strings.Contains(rendered, "  - Result: blocks") ||
		!strings.Contains(rendered, "  - Default: Use the existing repair command.") {
		t.Fatalf("rendered observations missing:\n%s", rendered)
	}
}

func TestRenderRoundTripsHardenObservationSkeleton(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:        "round-1",
		Status:    string(spec.HardenInProgress),
		StartedAt: "2026-05-04T00:00:00Z",
		Observations: []spec.HardenObservation{
			{Dimension: "path"},
			{Dimension: "command"},
		},
	}}
	rendered := string(Render(model))
	for _, want := range []string{"- path\n  - Result: \n  - Anchor: ", "- command\n  - Result: "} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered observation skeleton missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "Required spec edits: none") {
		t.Fatalf("in-progress harden scaffold should not seed required spec edits as none:\n%s", rendered)
	}
	parsed, err := Parse([]byte(rendered))
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.HardenRounds) != 1 || len(parsed.HardenRounds[0].Observations) != 2 {
		t.Fatalf("parsed observations = %+v", parsed.HardenRounds)
	}
	if got := parsed.HardenRounds[0].Observations[0]; got.Dimension != "path" || got.Anchor != "" || got.Result != "" || got.Note != "" {
		t.Fatalf("parsed skeleton observation = %+v", got)
	}
}

func TestParseNormalizesAlternateLivingSpecLabels(t *testing.T) {
	t.Parallel()

	input := string(Render(fixtureModel()))
	replacements := map[string]string{
		"Current phase: none": "current_phase: phase1",
		"Allowed follow-up command: `scafld status fixture-task`": "allowed_follow_up: `scafld review fixture-task`",
		"Latest runner update: none":                              "latest_runner_update: 2026-05-04T00:00:00Z",
		"Review gate: not_started":                                "review_gate: fail",
		"Profile: standard":                                       "profile: strict",
		"## Phase 1: Implementation\n\nStatus: pending":           "## Phase 1: Implementation\n\nstatus: active",
		"Objective: Build the slice.":                             "objective: Build the stricter slice.",
		"  - Expected kind: `exit_code_zero`":                     "  - expected_kind: `exit_code_zero`",
		"  - Status: pending":                                     "  - status: pass\n  - source_event: entry-1",
		"Created by: scafld":                                      "created_by: tester",
	}
	for old, newValue := range replacements {
		input = strings.Replace(input, old, newValue, 1)
	}

	parsed, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CurrentState.CurrentPhase != "phase1" || parsed.CurrentState.AllowedFollowUp != "scafld review fixture-task" || parsed.CurrentState.ReviewGate != "fail" {
		t.Fatalf("current state labels not normalized: %+v", parsed.CurrentState)
	}
	if parsed.Acceptance.ValidationProfile != "strict" {
		t.Fatalf("acceptance profile = %q", parsed.Acceptance.ValidationProfile)
	}
	if parsed.Origin.CreatedBy != "tester" {
		t.Fatalf("origin = %+v", parsed.Origin)
	}
	if got := parsed.Phases[0]; got.Status != "active" || got.Objective != "Build the stricter slice." {
		t.Fatalf("phase labels not normalized: %+v", got)
	}
	if got := parsed.Phases[0].Acceptance[0]; got.Status != "pass" || got.SourceEvent != "entry-1" {
		t.Fatalf("criterion labels not normalized: %+v", got)
	}

	rendered := string(Render(parsed))
	for _, bad := range []string{"current_phase:", "allowed_follow_up:", "latest_runner_update:", "review_gate:", "expected_kind:", "source_event:", "created_by:"} {
		if strings.Contains(rendered, bad) {
			t.Fatalf("render kept noncanonical label %q:\n%s", bad, rendered)
		}
	}
	for _, want := range []string{
		"Current phase: phase1",
		"Allowed follow-up command: `scafld review fixture-task`",
		"Latest runner update: 2026-05-04T00:00:00Z",
		"Review gate: fail",
		"  - Expected kind: `exit_code_zero`",
		"  - Source event: entry-1",
		"Created by: tester",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("render missing canonical label %q:\n%s", want, rendered)
		}
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

func TestRenderPreservesPhaseZeroNumber(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.Phases[0].ID = "phase0"
	model.Phases[0].Number = 0
	rendered := string(Render(model))
	if !strings.Contains(rendered, "## Phase 0: Implementation") {
		t.Fatalf("rendered phase zero with wrong number:\n%s", rendered)
	}
	if strings.Contains(rendered, "## Phase 1: Implementation") {
		t.Fatalf("rendered synthetic phase one for phase0:\n%s", rendered)
	}
}

func TestUpdateSpecMarkdownDoesNotAppendSyntheticPhaseForPhaseZero(t *testing.T) {
	t.Parallel()

	current := []byte(`---
spec_version: '2.0'
task_id: phase-zero
created: '2026-05-01T00:00:00Z'
updated: '2026-05-01T00:00:00Z'
status: active
harden_status: not_run
size: small
risk_level: low
---

# Phase zero task

## Current State

Status: active
Current phase: phase0
Next: build
Reason: phase phase0 opened
Blockers: none
Allowed follow-up command: ` + "`" + `scafld handoff phase-zero` + "`" + `
Latest runner update: 2026-05-01T00:00:00Z
Review gate: not_started

## Summary

Use a zero-indexed phase.

## Phase 0: Confirm the golden net

- Keep this phase body.

## Phase 1: Implement

- Keep this implementation body.
`)
	previous, err := Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	next := previous
	next.Phases = append([]spec.Phase(nil), previous.Phases...)
	next.Phases[0].Status = "completed"
	updated, err := updateSpecMarkdown(current, previous, next)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	if strings.Contains(text, "## Phase 2: Confirm the golden net") {
		t.Fatalf("phase0 save appended synthetic phase:\n%s", text)
	}
	if got := strings.Count(text, "## Phase "); got != 2 {
		t.Fatalf("phase count = %d, want 2:\n%s", got, text)
	}
}

func TestUpdateSpecMarkdownPreservesLiteratePhaseBodyWhenStatusChanges(t *testing.T) {
	t.Parallel()

	current := []byte(`---
spec_version: '2.0'
task_id: literate-phase
created: '2026-05-01T00:00:00Z'
updated: '2026-05-01T00:00:00Z'
status: active
harden_status: not_run
size: small
risk_level: low
---

# Literate phase task

## Current State

Status: active
Current phase: phase1
Next: build
Reason: phase phase1 opened
Blockers: none
Allowed follow-up command: ` + "`" + `scafld handoff literate-phase` + "`" + `
Latest runner update: 2026-05-01T00:00:00Z
Review gate: not_started

## Summary

Keep phase prose stable.

## Phase 1: Implementation

- Introduce the registry.
- Preserve the concrete implementation contract.
`)
	previous, err := Parse(current)
	if err != nil {
		t.Fatal(err)
	}
	next := previous
	next.Phases = append([]spec.Phase(nil), previous.Phases...)
	next.Phases[0].Status = "completed"
	updated, err := updateSpecMarkdown(current, previous, next)
	if err != nil {
		t.Fatal(err)
	}
	text := string(updated)
	for _, want := range []string{
		"Status: completed",
		"- Introduce the registry.",
		"- Preserve the concrete implementation contract.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("updated spec missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Changes:\n- none") || strings.Contains(text, "Objective: Complete this phase.") {
		t.Fatalf("literate phase was reset to canonical boilerplate:\n%s", text)
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

func TestSpecStoreListAllIncludesArchivedSpecs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	model := fixtureModel()
	path, err := store.CreateDraft(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	model.Status = spec.StatusCompleted
	model.Updated = "2026-05-05T01:00:00Z"
	if err := store.Save(context.Background(), path, model); err != nil {
		t.Fatal(err)
	}
	current, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 0 {
		t.Fatalf("List records = %+v, want no current records", current)
	}
	all, err := store.ListAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].TaskID != "fixture-task" || all[0].Status != spec.StatusCompleted {
		t.Fatalf("ListAll records = %+v", all)
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

func TestLoadHealsLocationDrift(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := Store{Root: root}
	model := fixtureModel()
	model.Status = spec.StatusApproved

	// Simulate a spec written by an older scafld version that did not relocate
	// files: status=approved in frontmatter, but the file lives under drafts/.
	draftsDir := filepath.Join(root, ".scafld", "specs", "drafts")
	if err := os.MkdirAll(draftsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	driftedPath := filepath.Join(draftsDir, model.TaskID+".md")
	if err := os.WriteFile(driftedPath, Render(model), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, path, err := store.Load(context.Background(), model.TaskID)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded.Status != spec.StatusApproved {
		t.Fatalf("loaded status = %s, want approved", loaded.Status)
	}
	approvedPath := filepath.Join(root, ".scafld", "specs", "approved", model.TaskID+".md")
	if path != approvedPath {
		t.Fatalf("returned path = %s, want %s", path, approvedPath)
	}
	if _, err := os.Stat(driftedPath); !os.IsNotExist(err) {
		t.Fatalf("drifted path still exists: %v", err)
	}
	if _, err := os.Stat(approvedPath); err != nil {
		t.Fatalf("approved path missing after heal: %v", err)
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
