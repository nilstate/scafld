package report

import (
	"context"
	"errors"
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
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

func TestRunReportsReviewDossierQualityMetrics(t *testing.T) {
	t.Parallel()

	discover := corereview.Dossier{
		Verdict: corereview.VerdictFail,
		Mode:    corereview.ModeDiscover,
		Summary: "Found one completion blocker and one non-blocking issue.",
		Findings: []corereview.Finding{
			{
				ID:               "missing-status-json",
				Severity:         corereview.SeverityHigh,
				BlocksCompletion: true,
				Confidence:       corereview.ConfidenceHigh,
				Location:         &corereview.Location{Path: "internal/app/status/status.go", Line: 42},
				Evidence:         "status output omits the active review blocker",
				Impact:           "agents cannot repair the current review failure without opening diagnostics manually",
				Validation:       "go test ./internal/app/status",
				Summary:          "status omits the latest blocker",
			},
			{
				ID:               "thin-doc-example",
				Severity:         corereview.SeverityLow,
				BlocksCompletion: false,
				Confidence:       corereview.ConfidenceMedium,
				Summary:          "docs would benefit from a richer report example",
			},
		},
		AttackLog: []corereview.AttackLogEntry{
			{Target: "status", Attack: "check repair payload", Result: corereview.AttackResultFinding},
			{Target: "docs", Attack: "check examples", Result: corereview.AttackResultClean},
		},
	}
	verify := corereview.Dossier{
		Verdict: corereview.VerdictPass,
		Mode:    corereview.ModeVerify,
		Summary: "Prior blocker fixed.",
		Findings: []corereview.Finding{
			{
				ID:               "missing-status-json",
				Severity:         corereview.SeverityHigh,
				BlocksCompletion: true,
				Status:           corereview.FindingFixed,
				Confidence:       corereview.ConfidenceHigh,
				Summary:          "status now includes the latest blocker",
			},
		},
		AttackLog: []corereview.AttackLogEntry{
			{Target: "status", Attack: "re-run repair payload check", Result: corereview.AttackResultClean},
		},
	}
	ledger := session.New("dossier-quality", "now")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "codex", Output: corereview.EncodeDossier(discover)})
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: "pass", Provider: "claude", Output: corereview.EncodeDossier(verify)})
	raw := session.New("raw-review", "now")
	raw = raw.WithEntry(session.Entry{Type: "review", Status: "fail", Provider: "codex", Output: `{"verdict":"fail"}`})

	out, err := Run(context.Background(), fakeSpecStore{}, fakeSessionStore{ledgers: []session.Session{ledger, raw}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Metrics.ReviewTotal != 3 || out.Metrics.ReviewDossierTotal != 2 || out.Metrics.ReviewDossierCoverage != 2.0/3.0 {
		t.Fatalf("dossier coverage metrics = %+v", out.Metrics)
	}
	if out.Metrics.ReviewFindingsTotal != 3 || out.Metrics.ReviewOpenBlockersTotal != 1 || out.Metrics.ReviewAttackAnglesTotal != 3 {
		t.Fatalf("dossier quality metrics = %+v", out.Metrics)
	}
	if out.Metrics.ReviewModeDistribution["discover"] != 1 || out.Metrics.ReviewModeDistribution["verify"] != 1 {
		t.Fatalf("mode distribution = %+v", out.Metrics.ReviewModeDistribution)
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
