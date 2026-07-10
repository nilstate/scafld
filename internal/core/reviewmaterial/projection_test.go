package reviewmaterial

import (
	"reflect"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/testkit/sessiontest"
)

func TestProjectClassifiesTaskMaterial(t *testing.T) {
	t.Parallel()

	reviewEntry := sessiontest.PassingReviewEntry("review-pass", "codex")
	reviewEntry.ReviewedScope = []string{"api"}
	reviewEntry.ReviewedMaterialDigest = "same-material"
	ledger := session.New("task", "2026-05-05T00:00:00Z").
		WithEntry(session.Entry{ID: "baseline", Type: session.EntryWorkspaceBaseline, Status: "captured", Output: " M old api/handler.go\n M old docs/index.md\n"}).
		WithEntry(reviewEntry)

	out := Project(Input{
		Model: spec.Model{
			TaskID: "task",
			Context: spec.Context{
				FilesImpacted: []string{"`api/handler.go`"},
			},
		},
		Ledger:                   ledger,
		CurrentSnapshot:          []string{" M new api/handler.go", " M new docs/index.md"},
		HasCurrentSnapshot:       true,
		Authority:                reviewgate.Authority{Found: true, Valid: true, ReviewEntry: reviewEntry},
		CurrentMaterialDigest:    "same-material",
		HasCurrentMaterialDigest: true,
	})

	if out.MaterialStatus != StatusUnchanged {
		t.Fatalf("material status = %q", out.MaterialStatus)
	}
	if !reflect.DeepEqual(out.Scope, []string{"api/handler.go"}) {
		t.Fatalf("scope = %+v", out.Scope)
	}
	if !reflect.DeepEqual(out.TaskChanges, []string{"changed api/handler.go (M old -> M new)"}) {
		t.Fatalf("task changes = %+v", out.TaskChanges)
	}
	if !reflect.DeepEqual(out.AmbientDrift, []string{"changed docs/index.md (M old -> M new)"}) {
		t.Fatalf("ambient drift = %+v", out.AmbientDrift)
	}
}

func TestProjectMarksLegacyReviewMaterialUnavailable(t *testing.T) {
	t.Parallel()

	reviewEntry := sessiontest.PassingReviewEntry("review-pass", "codex")
	out := Project(Input{
		Model: spec.Model{
			TaskID: "task",
			Context: spec.Context{
				FilesImpacted: []string{"`api/handler.go`"},
			},
		},
		Ledger: session.New("task", "2026-05-05T00:00:00Z").
			WithEntry(session.Entry{ID: "baseline", Type: session.EntryWorkspaceBaseline, Status: "captured", Output: " M old api/handler.go\n"}).
			WithEntry(reviewEntry),
		CurrentSnapshot:    []string{" M old api/handler.go"},
		HasCurrentSnapshot: true,
		Authority:          reviewgate.Authority{Found: true, Valid: true, ReviewEntry: reviewEntry},
	})

	if out.MaterialStatus != StatusReviewedMaterialUnavailable {
		t.Fatalf("material status = %q", out.MaterialStatus)
	}
}
