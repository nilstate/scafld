package approve

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecs struct{ model spec.Model }

func (f *fakeSpecs) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}

func (f *fakeSpecs) Save(_ context.Context, _ string, model spec.Model) error {
	f.model = model
	return nil
}

func (f *fakeSpecs) Find(string) (string, error) {
	return "approved/task.md", nil
}

type fakeSessions struct{ entries []session.Entry }

func (f *fakeSessions) Append(_ context.Context, _ string, entry session.Entry, _ string) (session.Session, error) {
	f.entries = append(f.entries, entry)
	ledger := session.New("task", "now")
	for _, item := range f.entries {
		ledger = ledger.WithEntry(item)
	}
	return ledger, nil
}

type fakeWorkspace struct{ snapshot []string }

func (f fakeWorkspace) ChangedFiles(context.Context) ([]string, error) {
	return append([]string(nil), f.snapshot...), nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC) }

func TestApproveRequiresDraft(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusApproved}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeClock{}, "task", "")
	if !errors.Is(err, ErrSpecNotDraft) {
		t.Fatalf("error = %v, want %v", err, ErrSpecNotDraft)
	}
}

func TestApproveAppendsSessionBeforeSavingSpec(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Status: spec.StatusDraft}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, fakeWorkspace{snapshot: []string{" M hash preexisting.go"}}, fakeClock{}, "task", "")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusApproved || specs.model.Status != spec.StatusApproved {
		t.Fatalf("output=%+v model=%+v", out, specs.model)
	}
	if len(sessions.entries) != 2 ||
		sessions.entries[0].Type != session.EntryWorkspaceBaseline ||
		sessions.entries[0].Status != "captured" ||
		!strings.Contains(sessions.entries[0].Output, "preexisting.go") ||
		sessions.entries[1].Type != "approval" ||
		sessions.entries[1].Status != "approved" {
		t.Fatalf("entries = %+v", sessions.entries)
	}
}

func TestApproveRequiresReasonForHardenNeedsRevision(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID:       "task",
		Status:       spec.StatusDraft,
		HardenStatus: spec.HardenNeedsRevision,
		HardenRounds: []spec.HardenRound{{
			ID:         "round-1",
			Status:     string(spec.HardenNeedsRevision),
			SpecDigest: "digest",
			Summary:    "bookkeeping surface request",
		}},
	}
	specs := &fakeSpecs{model: model}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeClock{}, "task", "")
	if !errors.Is(err, ErrApprovalReasonRequired) {
		t.Fatalf("error = %v, want %v", err, ErrApprovalReasonRequired)
	}
	if specs.model.Status != spec.StatusDraft {
		t.Fatalf("spec was saved despite missing reason: %+v", specs.model)
	}
}

func TestApproveRequiresReasonForDigestlessPassedHardenRound(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Version:      "2.0",
		TaskID:       "task",
		Status:       spec.StatusDraft,
		Title:        "Task",
		Summary:      "Original summary",
		HardenStatus: spec.HardenPassed,
		HardenRounds: []spec.HardenRound{{
			ID:      "round-1",
			Status:  string(spec.HardenPassed),
			Summary: "legacy pass without digest",
		}},
	}
	specs := &fakeSpecs{model: model}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeClock{}, "task", "")
	if !errors.Is(err, ErrApprovalReasonRequired) {
		t.Fatalf("error = %v, want %v", err, ErrApprovalReasonRequired)
	}
	if specs.model.Status != spec.StatusDraft {
		t.Fatalf("spec was saved despite missing reason: %+v", specs.model)
	}
}

func TestApproveRequiresReasonForLegacyPassedHardenStatusWithoutRound(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Version:      "2.0",
		TaskID:       "task",
		Status:       spec.StatusDraft,
		Title:        "Task",
		Summary:      "Original summary",
		HardenStatus: spec.HardenPassed,
	}
	specs := &fakeSpecs{model: model}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeClock{}, "task", "")
	if !errors.Is(err, ErrApprovalReasonRequired) {
		t.Fatalf("error = %v, want %v", err, ErrApprovalReasonRequired)
	}
}

func TestApproveWithReasonRecordsHardenOverride(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID:       "task",
		Status:       spec.StatusDraft,
		HardenStatus: spec.HardenNeedsRevision,
		HardenRounds: []spec.HardenRound{{
			ID:         "round-1",
			Status:     string(spec.HardenNeedsRevision),
			SpecDigest: "digest",
			Summary:    "surface bookkeeping",
		}},
	}
	specs := &fakeSpecs{model: model}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, nil, fakeClock{}, "task", "operator rejects surface matrix as overengineering")
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != spec.StatusApproved || specs.model.Status != spec.StatusApproved {
		t.Fatalf("output=%+v model=%+v", out, specs.model)
	}
	if specs.model.HardenStatus != spec.HardenOverridden {
		t.Fatalf("harden status = %s, want overridden", specs.model.HardenStatus)
	}
	round := specs.model.HardenRounds[0]
	if round.Status != string(spec.HardenOverridden) || round.EndedAt == "" || !strings.Contains(round.Summary, "operator rejects surface matrix") {
		t.Fatalf("round = %+v", round)
	}
	if len(sessions.entries) != 2 ||
		sessions.entries[0].Type != "harden_override" ||
		sessions.entries[0].Status != "accepted" ||
		sessions.entries[0].Provider != "human" ||
		sessions.entries[1].Type != "approval" ||
		!strings.Contains(sessions.entries[1].Reason, "operator rejects surface matrix") {
		t.Fatalf("entries = %+v", sessions.entries)
	}
	var payload struct {
		Kind    string `json:"kind"`
		RoundID string `json:"round_id"`
	}
	if err := json.Unmarshal([]byte(sessions.entries[0].Output), &payload); err != nil {
		t.Fatalf("override output is not JSON: %v\n%s", err, sessions.entries[0].Output)
	}
	if payload.Kind != "needs_operator_decision" || payload.RoundID != "round-1" {
		t.Fatalf("override payload = %+v", payload)
	}
}
