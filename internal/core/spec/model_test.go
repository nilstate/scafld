package spec

import (
	"errors"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
)

func TestErrorWrappingAndErrorClassification(t *testing.T) {
	t.Parallel()

	validation := Validate(Model{Version: "bad"})
	if validation.Valid {
		t.Fatal("invalid model accepted")
	}
	if !errors.Is(validation, ErrInvalidSpec) {
		t.Fatalf("validation should wrap ErrInvalidSpec: %v", validation)
	}
}

func TestCriterionEvidenceValidation(t *testing.T) {
	t.Parallel()

	model := Model{
		Version: "2.0",
		TaskID:  "task",
		Title:   "Task",
		Status:  StatusDraft,
		Phases: []Phase{{ID: "phase1", Name: "Phase", Acceptance: []Criterion{{
			ID:           "ac1",
			Command:      "true",
			ExpectedKind: acceptance.ExpectedExitCodeZero,
		}}}},
	}
	if validation := Validate(model); !validation.Valid {
		t.Fatalf("model should validate: %+v", validation.Errors)
	}
}

func TestBrowserCriterionRequiresBrowserEvidenceExpectedKind(t *testing.T) {
	t.Parallel()

	model := Model{
		Version: "2.0",
		TaskID:  "task",
		Title:   "Task",
		Status:  StatusDraft,
		Phases: []Phase{{ID: "phase1", Name: "Phase", Acceptance: []Criterion{{
			ID:           "ac1",
			Type:         "browser",
			Command:      "npm run browser:check",
			ExpectedKind: acceptance.ExpectedExitCodeZero,
		}}}},
	}
	validation := Validate(model)
	if validation.Valid {
		t.Fatal("browser criterion with exit_code_zero accepted")
	}
	if !containsValidationError(validation.Errors, "criterion ac1 browser type requires expected_kind browser_evidence") {
		t.Fatalf("validation errors = %+v", validation.Errors)
	}
	model.Phases[0].Acceptance[0].ExpectedKind = acceptance.ExpectedBrowserEvidence
	if validation := Validate(model); !validation.Valid {
		t.Fatalf("browser criterion should validate: %+v", validation.Errors)
	}
	model.Phases[0].Acceptance[0].Type = "command"
	validation = Validate(model)
	if validation.Valid {
		t.Fatal("browser_evidence with command type accepted")
	}
	if !containsValidationError(validation.Errors, "criterion ac1 browser_evidence expected_kind requires browser type") {
		t.Fatalf("validation errors = %+v", validation.Errors)
	}
}

func TestManualCriterionRequiresManualExpectedKindAndNoCommand(t *testing.T) {
	t.Parallel()

	model := Model{
		Version: "2.0",
		TaskID:  "task",
		Title:   "Task",
		Status:  StatusDraft,
		Phases: []Phase{{ID: "phase1", Name: "Phase", Acceptance: []Criterion{{
			ID:           "ac1",
			Type:         "manual",
			Title:        "Operator signoff",
			ExpectedKind: acceptance.ExpectedExitCodeZero,
		}}}},
	}
	validation := Validate(model)
	if validation.Valid {
		t.Fatal("manual criterion with exit_code_zero accepted")
	}
	if !containsValidationError(validation.Errors, "criterion ac1 manual type requires expected_kind manual") {
		t.Fatalf("validation errors = %+v", validation.Errors)
	}
	model.Phases[0].Acceptance[0].ExpectedKind = acceptance.ExpectedManual
	if validation := Validate(model); !validation.Valid {
		t.Fatalf("manual criterion should validate: %+v", validation.Errors)
	}
	model.Phases[0].Acceptance[0].Command = "true"
	validation = Validate(model)
	if validation.Valid {
		t.Fatal("manual criterion with command accepted")
	}
	if !containsValidationError(validation.Errors, "criterion ac1 manual type cannot have a command") {
		t.Fatalf("validation errors = %+v", validation.Errors)
	}
	model.Phases[0].Acceptance[0].Type = "command"
	model.Phases[0].Acceptance[0].Command = "true"
	validation = Validate(model)
	if validation.Valid {
		t.Fatal("manual expected_kind with command type accepted")
	}
	if !containsValidationError(validation.Errors, "criterion ac1 manual expected_kind requires manual type") {
		t.Fatalf("validation errors = %+v", validation.Errors)
	}
}

func TestInvalidExpectedKindNamesSupportedValuesAndHint(t *testing.T) {
	t.Parallel()

	model := Model{
		Version: "2.0",
		TaskID:  "task",
		Title:   "Task",
		Status:  StatusDraft,
		Phases: []Phase{{ID: "phase1", Name: "Phase", Acceptance: []Criterion{{
			ID:           "ac1",
			Command:      "rg forbidden",
			ExpectedKind: acceptance.ExpectedKind("stdout_empty"),
		}}}},
	}
	validation := Validate(model)
	if validation.Valid {
		t.Fatal("invalid expected_kind accepted")
	}
	joined := strings.Join(validation.Errors, "\n")
	for _, want := range []string{`expected_kind "stdout_empty" is invalid`, "supported values: exit_code_zero, exit_code_nonzero, no_matches, manual, browser_evidence", "use no_matches when stdout must be empty"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("validation errors missing %q: %+v", want, validation.Errors)
		}
	}
}

func TestHardenContractDigestIgnoresHardenLifecycleButTracksDraftEdits(t *testing.T) {
	t.Parallel()

	model := Model{
		Version:      "2.0",
		TaskID:       "task",
		Title:        "Task",
		Summary:      "Original summary",
		Status:       StatusDraft,
		HardenStatus: HardenNotRun,
		CurrentState: CurrentState{AllowedFollowUp: "scafld approve task"},
		Acceptance: Acceptance{
			DefinitionDone: []ChecklistItem{{ID: "dod1", Text: "Behavior is covered"}},
			Criteria: []Criterion{{
				ID:           "global",
				Title:        "Full check",
				Command:      "make check",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pending",
			}},
		},
		Phases: []Phase{{ID: "phase1", Number: 1, Name: "Phase"}},
	}
	base := HardenContractDigest(model)
	if base == "" {
		t.Fatal("digest is empty")
	}
	lifecycleOnly := model
	lifecycleOnly.HardenStatus = HardenNeedsRevision
	lifecycleOnly.CurrentState.Blockers = "one blocker"
	lifecycleOnly.HardenRounds = []HardenRound{{ID: "round-1", Status: string(HardenNeedsRevision), Summary: "found issue"}}
	lifecycleOnly.Acceptance.DefinitionDone[0].Checked = true
	lifecycleOnly.Acceptance.Criteria[0].Status = "pass"
	lifecycleOnly.Acceptance.Criteria[0].Evidence = "entry-1"
	lifecycleOnly.Acceptance.Criteria[0].SourceEvent = "entry-1"
	if got := HardenContractDigest(lifecycleOnly); got != base {
		t.Fatalf("harden lifecycle changed digest: %s != %s", got, base)
	}
	revised := lifecycleOnly
	revised.Summary = "Revised summary"
	if got := HardenContractDigest(revised); got == base {
		t.Fatalf("draft edit did not change digest: %s", got)
	}
	revisedCommand := lifecycleOnly
	revisedCommand.Acceptance.Criteria[0].Command = "make check && go test ./..."
	if got := HardenContractDigest(revisedCommand); got == base {
		t.Fatalf("acceptance command edit did not change digest: %s", got)
	}
}

func TestContractDigestIgnoresLifecycleEvidenceAndProvenanceButTracksContractEdits(t *testing.T) {
	t.Parallel()

	model := Model{
		Version:      "2.0",
		TaskID:       "task",
		Created:      "2026-07-21T00:00:00Z",
		Updated:      "2026-07-21T00:00:00Z",
		Title:        "Task",
		Summary:      "Original summary",
		Status:       StatusReview,
		HardenStatus: HardenPassed,
		CurrentState: CurrentState{ReviewGate: "passed"},
		Acceptance: Acceptance{
			DefinitionDone: []ChecklistItem{{ID: "dod1", Text: "Behavior is covered"}},
			Criteria: []Criterion{{
				ID:           "global",
				Title:        "Full check",
				Command:      "make check",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pending",
			}},
		},
		Phases: []Phase{{
			ID:             "phase1",
			Number:         1,
			Name:           "Phase",
			Status:         "completed",
			Reason:         "done",
			Objective:      "Ship the behavior",
			Changes:        []string{"Update core path"},
			DefinitionDone: []ChecklistItem{{ID: "phase-dod1", Text: "Phase behavior is covered"}},
			Acceptance: []Criterion{{
				ID:           "phase-ac1",
				Title:        "Phase check",
				Command:      "go test ./internal/core/...",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
				Status:       "pending",
			}},
		}},
		Review: ReviewState{Status: "passed", Verdict: "pass", Summary: "clean"},
		Origin: Origin{CreatedBy: "scafld", Source: "plan"},
		HardenRounds: []HardenRound{{
			ID:         "round-1",
			Status:     string(HardenPassed),
			SpecDigest: "old",
			Summary:    "passed",
		}},
		PlanningLog: []PlanningEvent{{Time: "2026-07-21T00:00:00Z", Text: "created"}},
	}
	base := ContractDigest(model)
	if base == "" {
		t.Fatal("digest is empty")
	}
	lifecycleOnly := model
	lifecycleOnly.Updated = "2026-07-21T01:00:00Z"
	lifecycleOnly.Status = StatusCompleted
	lifecycleOnly.HardenStatus = HardenNeedsRevision
	lifecycleOnly.CurrentState = CurrentState{Next: string(StatusCompleted), Reason: "complete"}
	lifecycleOnly.Acceptance.DefinitionDone[0].Checked = true
	lifecycleOnly.Acceptance.Criteria[0].Status = "pass"
	lifecycleOnly.Acceptance.Criteria[0].Evidence = "exit code was 0"
	lifecycleOnly.Acceptance.Criteria[0].SourceEvent = "entry-1"
	lifecycleOnly.Phases[0].Status = "active"
	lifecycleOnly.Phases[0].Reason = "rerunning"
	lifecycleOnly.Phases[0].DefinitionDone[0].Checked = true
	lifecycleOnly.Phases[0].Acceptance[0].Status = "pass"
	lifecycleOnly.Phases[0].Acceptance[0].Evidence = "exit code was 0"
	lifecycleOnly.Phases[0].Acceptance[0].SourceEvent = "entry-2"
	lifecycleOnly.Review.Summary = "updated review projection"
	lifecycleOnly.Origin = Origin{CreatedBy: "test", Source: "render"}
	lifecycleOnly.HardenRounds[0].Summary = "updated harden projection"
	lifecycleOnly.PlanningLog = append(lifecycleOnly.PlanningLog, PlanningEvent{Time: "2026-07-21T01:00:00Z", Text: "saved"})
	if got := ContractDigest(lifecycleOnly); got != base {
		t.Fatalf("lifecycle evidence changed digest: %s != %s", got, base)
	}
	editedCommand := lifecycleOnly
	editedCommand.Acceptance.Criteria[0].Command = "make check && go test ./..."
	if got := ContractDigest(editedCommand); got == base {
		t.Fatalf("acceptance command edit did not change digest: %s", got)
	}
	editedExpected := lifecycleOnly
	editedExpected.Phases[0].Acceptance[0].ExpectedKind = acceptance.ExpectedExitCodeNonzero
	if got := ContractDigest(editedExpected); got == base {
		t.Fatalf("expected kind edit did not change digest: %s", got)
	}
}

func containsValidationError(errors []string, want string) bool {
	for _, err := range errors {
		if err == want {
			return true
		}
	}
	return false
}
