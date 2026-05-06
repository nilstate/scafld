package validate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecStore struct {
	model spec.Model
	path  string
	err   error
}

func (f fakeSpecStore) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, f.path, f.err
}

func validModel() spec.Model {
	return spec.Model{
		Version: "2.0",
		TaskID:  "task",
		Title:   "Task",
		Status:  spec.StatusDraft,
		Phases: []spec.Phase{{
			ID:   "phase1",
			Name: "Phase",
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				Command:      "true",
				ExpectedKind: acceptance.ExpectedExitCodeZero,
			}},
		}},
	}
}

func TestRunReturnsValidSpecProjection(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), fakeSpecStore{model: validModel(), path: "drafts/task.md"}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Valid || out.TaskID != "task" || out.Path != "drafts/task.md" || len(out.Errors) != 0 {
		t.Fatalf("output = %+v", out)
	}
}

func TestRunReturnsValidationErrors(t *testing.T) {
	t.Parallel()

	model := validModel()
	model.Title = ""
	model.Phases[0].Acceptance[0].ExpectedKind = "unset"
	out, err := Run(context.Background(), fakeSpecStore{model: model, path: "drafts/task.md"}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Valid || len(out.Errors) == 0 {
		t.Fatalf("output = %+v, want invalid", out)
	}
	joined := strings.Join(out.Errors, "\n")
	if !strings.Contains(joined, "title is required") || !strings.Contains(joined, "expected_kind") {
		t.Fatalf("errors = %v", out.Errors)
	}
}

func TestRunWrapsLoadError(t *testing.T) {
	t.Parallel()

	want := errors.New("missing spec")
	_, err := Run(context.Background(), fakeSpecStore{err: want}, "task")
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "load spec") {
		t.Fatalf("error = %v, want load spec wrapping %v", err, want)
	}
}
