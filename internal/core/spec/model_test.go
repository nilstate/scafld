package spec

import (
	"errors"
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
