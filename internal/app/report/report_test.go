package report

import (
	"context"
	"errors"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

type fakeSpecStore struct {
	records []spec.Record
	all     []spec.Record
	err     error
}

func (f fakeSpecStore) List(context.Context) ([]spec.Record, error) {
	return f.records, f.err
}

func (f fakeSpecStore) ListAll(context.Context) ([]spec.Record, error) {
	return f.all, f.err
}

type fakeSessionStore struct {
	ledgers []session.Session
	err     error
}

func (f fakeSessionStore) List(context.Context) ([]session.Session, error) {
	return f.ledgers, f.err
}

func TestRunCountsSpecsByStatus(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), fakeSpecStore{all: []spec.Record{
		{TaskID: "draft", Status: spec.StatusDraft},
		{TaskID: "active", Status: spec.StatusActive},
		{TaskID: "review", Status: spec.StatusReview},
		{TaskID: "review-two", Status: spec.StatusReview},
	}}, nil)
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

func TestRunReportsSessionDerivedMetrics(t *testing.T) {
	t.Parallel()

	firstPass := session.New("first-pass", "now")
	firstPass = firstPass.WithEntry(session.Entry{Type: session.EntryWorkspaceBaseline, Status: "captured"})
	firstPass = firstPass.WithEntry(session.Entry{Type: "build", Status: "active"})
	firstPass = firstPass.WithEntry(session.Entry{Type: "build", Status: string(spec.StatusReview)})
	firstPass = firstPass.WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "codex"})

	recovered := session.New("recovered", "now")
	recovered = recovered.WithEntry(session.Entry{Type: "build", Status: "active"})
	recovered = recovered.WithEntry(session.Entry{Type: "build", Status: string(spec.StatusBlocked)})
	recovered = recovered.WithEntry(session.Entry{Type: "build", Status: "active"})
	recovered = recovered.WithEntry(session.Entry{Type: "build", Status: string(spec.StatusReview)})
	recovered = recovered.WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "claude"})
	recovered = recovered.WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "codex"})

	stillBlocked := session.New("blocked", "now")
	stillBlocked = stillBlocked.WithEntry(session.Entry{Type: "build", Status: "blocked"})
	stillBlocked = stillBlocked.WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "codex"})
	stillBlocked = stillBlocked.WithEntry(session.Entry{Type: "complete", Status: "completed"})

	out, err := Run(context.Background(), fakeSpecStore{}, fakeSessionStore{ledgers: []session.Session{firstPass, recovered, stillBlocked}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metrics.FirstAttemptTotal != 3 || out.Metrics.FirstAttemptPasses != 1 || out.Metrics.FirstAttemptPassRate != 1.0/3.0 {
		t.Fatalf("first attempt metrics = %+v", out.Metrics)
	}
	if out.Metrics.RecoveryTotal != 2 || out.Metrics.RecoveredTasks != 1 || out.Metrics.RecoveryConvergenceRate != 0.5 {
		t.Fatalf("recovery metrics = %+v", out.Metrics)
	}
	if out.Metrics.ReviewTotal != 4 || out.Metrics.ReviewPasses != 2 || out.Metrics.ReviewPassRate != 0.5 {
		t.Fatalf("review metrics = %+v", out.Metrics)
	}
	if out.Metrics.ReviewChallengeTotal != 2 || out.Metrics.ChallengeOverrides != 1 || out.Metrics.ChallengeOverrideRate != 0.5 {
		t.Fatalf("challenge metrics = %+v", out.Metrics)
	}
	if out.Metrics.SessionTotal != 3 || out.Metrics.WorkspaceBaselineTasks != 1 || out.Metrics.WorkspaceBaselineCoverage != 1.0/3.0 {
		t.Fatalf("baseline metrics = %+v", out.Metrics)
	}
	if out.Metrics.BlockedGateDistribution["build"] != 2 || out.Metrics.BlockedGateDistribution["review"] != 2 {
		t.Fatalf("blocked gate distribution = %+v", out.Metrics.BlockedGateDistribution)
	}
}

func TestRunTreatsUnknownReviewProviderAsChallengeOverride(t *testing.T) {
	t.Parallel()

	ledger := session.New("override", "now")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "unknown"})
	ledger = ledger.WithEntry(session.Entry{Type: "complete", Status: "completed"})

	out, err := Run(context.Background(), fakeSpecStore{}, fakeSessionStore{ledgers: []session.Session{ledger}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metrics.ReviewChallengeTotal != 1 || out.Metrics.ChallengeOverrides != 1 {
		t.Fatalf("challenge metrics = %+v", out.Metrics)
	}
}

func TestRunCountsHumanReviewedReviewAsChallengeOverride(t *testing.T) {
	t.Parallel()

	ledger := session.New("override", "now")
	ledger = ledger.WithEntry(session.Entry{Type: "review_override", Status: "accepted", Provider: "human", Reason: "operator reviewed PR"})
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "human"})
	ledger = ledger.WithEntry(session.Entry{Type: "complete", Status: "completed"})

	out, err := Run(context.Background(), fakeSpecStore{}, fakeSessionStore{ledgers: []session.Session{ledger}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metrics.ReviewChallengeTotal != 1 || out.Metrics.ChallengeOverrides != 1 || out.Metrics.ReviewPasses != 1 {
		t.Fatalf("challenge metrics = %+v", out.Metrics)
	}
}

func TestRunPropagatesStoreError(t *testing.T) {
	t.Parallel()

	want := errors.New("list failed")
	_, err := Run(context.Background(), fakeSpecStore{err: want}, nil)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestRunPropagatesSessionStoreError(t *testing.T) {
	t.Parallel()

	want := errors.New("session list failed")
	_, err := Run(context.Background(), fakeSpecStore{}, fakeSessionStore{err: want})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
