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

func containsValidationError(errors []string, want string) bool {
	for _, err := range errors {
		if err == want {
			return true
		}
	}
	return false
}
