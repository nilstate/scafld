package report

import (
	"context"
	"errors"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecStore struct {
	records []spec.Record
	err     error
}

func (f fakeSpecStore) List(context.Context) ([]spec.Record, error) {
	return f.records, f.err
}

func TestRunCountsSpecsByStatus(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), fakeSpecStore{records: []spec.Record{
		{TaskID: "draft", Status: spec.StatusDraft},
		{TaskID: "active", Status: spec.StatusActive},
		{TaskID: "review", Status: spec.StatusReview},
		{TaskID: "review-two", Status: spec.StatusReview},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Total != 4 {
		t.Fatalf("total = %d, want 4", out.Total)
	}
	if out.ByStatus[spec.StatusDraft] != 1 || out.ByStatus[spec.StatusActive] != 1 || out.ByStatus[spec.StatusReview] != 2 {
		t.Fatalf("by status = %+v", out.ByStatus)
	}
}

func TestRunPropagatesStoreError(t *testing.T) {
	t.Parallel()

	want := errors.New("list failed")
	_, err := Run(context.Background(), fakeSpecStore{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
