package gate

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/app/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
)

// mutatingSnapshotter returns a different tree on its second call, simulating a
// scoped file rewritten by an acceptance command between the signed snapshot and
// the mutation-guard re-snapshot.
type mutatingSnapshotter struct{ calls int }

func (m *mutatingSnapshotter) Snapshot(context.Context, SnapshotInput) (Snapshot, error) {
	m.calls++
	tree := "tree-pre"
	if m.calls > 1 {
		tree = "tree-post-mutation"
	}
	return Snapshot{TreeSHA: tree, FileDigests: map[string]string{"f.go": "sha"}, IgnoredUnreviewed: []string{}}, nil
}

type fakeSnapshotter struct {
	snapshot Snapshot
	called   int
}

func (f *fakeSnapshotter) Snapshot(context.Context, SnapshotInput) (Snapshot, error) {
	f.called++
	return f.snapshot, nil
}

type fakeAcceptance struct {
	output acceptance.EvaluateOutput
	called int
}

func (f *fakeAcceptance) Evaluate(context.Context, acceptance.EvaluateInput) (acceptance.EvaluateOutput, error) {
	f.called++
	return f.output, nil
}

// defaultProvenance covers the single file in baseSnapshotter so a reviewer that
// does not set provenance still yields a coverage-complete receipt; tests that
// exercise the coverage guard pass a non-nil empty provenance to opt out.
var defaultProvenance = []receipt.Provenance{{Kind: "evidence_file", Path: "internal/app/gate/gate.go", SHA256: "sha"}}

type fakeReviewer struct {
	dossier    review.Dossier
	provenance []receipt.Provenance
	ignored    []string
	reviewer   receipt.Reviewer
	called     int
	input      ReviewInput
}

func (f *fakeReviewer) Review(_ context.Context, input ReviewInput) (ReviewResult, error) {
	f.called++
	f.input = input
	rev := f.reviewer
	if rev.Provider == "" {
		rev = receipt.Reviewer{Provider: "codex"}
	}
	prov := f.provenance
	if prov == nil {
		prov = defaultProvenance
	}
	return ReviewResult{Dossier: f.dossier, Provenance: prov, Ignored: f.ignored, Reviewer: rev}, nil
}

type fakeSigner struct {
	body   receipt.Body
	called int
}

func (f *fakeSigner) Sign(body receipt.Body) (receipt.DetachedSignature, error) {
	f.called++
	f.body = body
	return receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"}, nil
}

func TestRunFailsFastOnAcceptanceBeforeReviewer(t *testing.T) {
	t.Parallel()

	reviewer := &fakeReviewer{}
	out, err := Run(context.Background(), baseSnapshotter(), &fakeAcceptance{output: acceptance.EvaluateOutput{
		Results: []acceptance.CriterionResult{{ID: "ac1", Status: "fail", Reason: "red"}},
		Passed:  false,
	}}, reviewer, &fakeSigner{}, baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Reason != "acceptance failed before review" {
		t.Fatalf("output = %+v, want acceptance fail-fast", out)
	}
	if reviewer.called != 0 {
		t.Fatalf("reviewer called after failed acceptance")
	}
}

func TestRunDowngradesBlockingFindingWithoutValidationToAdvisory(t *testing.T) {
	t.Parallel()

	signer := &fakeSigner{}
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeReviewer{dossier: review.Dossier{
		Verdict: review.VerdictFail,
		Findings: []review.Finding{{
			ID:               "f1",
			Severity:         review.SeverityHigh,
			BlocksCompletion: true,
			Location:         &review.Location{Path: "internal/app/gate/gate.go", Line: 12},
			Summary:          "claimed blocker",
		}},
	}}, signer, baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass || out.Receipt == nil || signer.called != 1 {
		t.Fatalf("output = %+v signer=%d, want receipt after advisory downgrade", out, signer.called)
	}
	if len(out.Findings) != 1 || out.Findings[0].BlocksCompletion || out.Findings[0].Confidence != review.ConfidenceLow {
		t.Fatalf("findings = %+v, want nonblocking advisory", out.Findings)
	}
	if len(out.Receipt.Body.OpenBlockers) != 0 {
		t.Fatalf("open blockers = %+v, want none", out.Receipt.Body.OpenBlockers)
	}
}

func TestRunAdvisoryFindingsDoNotGate(t *testing.T) {
	t.Parallel()

	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeReviewer{dossier: review.Dossier{
		Verdict: review.VerdictPass,
		Findings: []review.Finding{{
			ID:               "note",
			Severity:         review.SeverityLow,
			BlocksCompletion: false,
			Summary:          "advisory note",
		}},
	}}, &fakeSigner{}, baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass || out.Receipt == nil {
		t.Fatalf("output = %+v, want passing receipt", out)
	}
}

func TestRunCleanPassMintsSignedReceiptAnchoredToLedgerHead(t *testing.T) {
	t.Parallel()

	signer := &fakeSigner{}
	input := baseInput()
	input.PriorLedgerHead = session.LedgerGenesisHead()
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, reviewer: receipt.Reviewer{Provider: "codex"}}, signer, input)
	if err != nil {
		t.Fatal(err)
	}
	if out.Receipt == nil || out.Receipt.Signature.Alg != receipt.SignatureAlgorithm {
		t.Fatalf("receipt = %+v", out.Receipt)
	}
	digest, err := receipt.ReceiptDigest(signer.body)
	if err != nil {
		t.Fatal(err)
	}
	wantHead := session.NextLedgerHead(input.PriorLedgerHead, digest)
	if out.Receipt.Body.LedgerHead != wantHead || signer.body.LedgerHead != wantHead {
		t.Fatalf("ledger head = %q signer=%q want %q", out.Receipt.Body.LedgerHead, signer.body.LedgerHead, wantHead)
	}
	if out.Receipt.Body.Reviewer.Provider != "codex" || out.Receipt.Body.Acceptance[0].ID != "ac1" {
		t.Fatalf("receipt body = %+v", out.Receipt.Body)
	}
}

func TestRunFailsClosedOnAcceptanceMutation(t *testing.T) {
	t.Parallel()

	reviewer := &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}
	signer := &fakeSigner{}
	out, err := Run(context.Background(), &mutatingSnapshotter{}, passingAcceptance(), reviewer, signer, baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Receipt != nil || !strings.Contains(out.Reason, "mutated") {
		t.Fatalf("expected fail-closed on acceptance mutation, got %+v", out)
	}
	if reviewer.called != 0 || signer.called != 0 {
		t.Fatalf("review/sign ran over a mutated tree: reviewer=%d signer=%d", reviewer.called, signer.called)
	}
}

func TestRunStampsReviewedContextProvenance(t *testing.T) {
	t.Parallel()

	prov := []receipt.Provenance{{Kind: "evidence_file", Path: "internal/app/gate/gate.go", SHA256: "sha", Bytes: 42}}
	reviewer := &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, provenance: prov}
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), reviewer, &fakeSigner{}, baseInput())
	if err != nil {
		t.Fatal(err)
	}
	if out.Receipt == nil || len(out.Receipt.Body.ReviewedContextProvenance) != 1 || out.Receipt.Body.ReviewedContextProvenance[0].Path != "internal/app/gate/gate.go" {
		t.Fatalf("reviewed-context provenance not stamped into receipt: %+v", out.Receipt)
	}
}

func TestRunFailsClosedOnUncoveredFileDigest(t *testing.T) {
	t.Parallel()

	// A reviewer that covers nothing (non-nil empty provenance opts out of the test
	// default) must not yield a signed receipt: the snapshot's file digest would be
	// neither reviewed nor declared ignored, so the gate fails closed at mint.
	signer := &fakeSigner{}
	reviewer := &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, provenance: []receipt.Provenance{}}
	_, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), reviewer, signer, baseInput())
	if err == nil || !strings.Contains(err.Error(), "not covered") {
		t.Fatalf("gate must fail closed minting an uncovered file digest, got err=%v", err)
	}
	if signer.called != 0 {
		t.Fatalf("an uncovered receipt must never be signed: signer called %d times", signer.called)
	}
}

func baseSnapshotter() *fakeSnapshotter {
	return &fakeSnapshotter{snapshot: Snapshot{
		TreeSHA:           "tree",
		BaseCommit:        "base",
		HeadCommit:        "head",
		FileDigests:       map[string]string{"internal/app/gate/gate.go": "sha"},
		IgnoredUnreviewed: []string{},
	}}
}

func passingAcceptance() *fakeAcceptance {
	return &fakeAcceptance{output: acceptance.EvaluateOutput{
		Results: []acceptance.CriterionResult{{
			ID:           "ac1",
			Command:      "go test ./internal/app/gate",
			ExpectedKind: "exit_code_zero",
			Status:       "pass",
			Reason:       "exit code was 0",
			StdoutDigest: "stdout-sha",
		}},
		Passed: true,
	}}
}

func baseInput() Input {
	return Input{
		TaskID:          "task",
		SessionID:       "session",
		Scope:           []string{"internal/app/gate"},
		SpecFingerprint: "spec",
		HostUnderReview: receipt.HostUnderReview{Agent: "codex"},
		Independence:    receipt.Independence{Level: "isolation_only"},
		Criteria:        []acceptance.Criterion{{ID: "ac1", Command: "go test ./internal/app/gate", ExpectedKind: "exit_code_zero"}},
		PriorLedgerHead: session.LedgerGenesisHead(),
		MintedAt:        time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
	}
}
