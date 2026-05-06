package list

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

func TestRunReturnsSpecRecordsInStoreOrder(t *testing.T) {
	t.Parallel()

	records := []spec.Record{
		{TaskID: "one", Status: spec.StatusDraft, Path: "drafts/one.md"},
		{TaskID: "two", Status: spec.StatusReview, Path: "active/two.md"},
	}
	got, err := Run(context.Background(), fakeSpecStore{records: records})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].TaskID != "one" || got[1].TaskID != "two" {
		t.Fatalf("records = %+v", got)
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
