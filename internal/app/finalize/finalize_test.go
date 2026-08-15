package finalize

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

type fakeSnapshotter struct {
	snapshot  Snapshot
	snapshots []Snapshot
	called    int
	input     SnapshotInput
}

func (f *fakeSnapshotter) Snapshot(_ context.Context, input SnapshotInput) (Snapshot, error) {
	snapshot := f.snapshot
	if len(f.snapshots) > 0 {
		index := f.called
		if index >= len(f.snapshots) {
			index = len(f.snapshots) - 1
		}
		snapshot = f.snapshots[index]
	}
	f.called++
	f.input = input
	return snapshot, nil
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
var defaultProvenance = []receipt.Provenance{{Kind: "evidence_file", Path: "internal/app/finalize/gate.go", SHA256: "sha"}}

type fakeReviewer struct {
	dossier    review.Dossier
	provenance []receipt.Provenance
	ignored    []string
	reviewer   receipt.Reviewer
}

func reviewInput(input Input, f *fakeReviewer) Input {
	rev := f.reviewer
	if rev.Provider == "" {
		rev = receipt.Reviewer{Provider: "codex"}
	}
	prov := f.provenance
	if prov == nil {
		prov = defaultProvenance
	}
	input.Review = ReviewEvidence{Dossier: f.dossier, Provenance: prov, Ignored: f.ignored, Reviewer: rev}
	return input
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

	out, err := Run(context.Background(), baseSnapshotter(), &fakeAcceptance{output: acceptance.EvaluateOutput{
		Results: []acceptance.CriterionResult{{ID: "ac1", Status: "fail", Reason: "red"}},
		Passed:  false,
	}}, &fakeSigner{}, reviewInput(baseInput(), &fakeReviewer{}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Reason != "acceptance failed before review" {
		t.Fatalf("output = %+v, want acceptance fail-fast", out)
	}
	if out.Independence.Level != receipt.IndependenceLevelIsolationOnly || out.Independence.Downgraded != receipt.IndependenceDowngradeSameVendor {
		t.Fatalf("failed acceptance independence = %+v, want isolation_only same_vendor", out.Independence)
	}
}

func TestRunDowngradesBlockingFindingWithoutValidationToAdvisory(t *testing.T) {
	t.Parallel()

	signer := &fakeSigner{}
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{
		Verdict: review.VerdictFail,
		Findings: []review.Finding{{
			ID:               "f1",
			Severity:         review.SeverityHigh,
			BlocksCompletion: true,
			Location:         &review.Location{Path: "internal/app/finalize/gate.go", Line: 12},
			Summary:          "claimed blocker",
		}},
	}}))
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

	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeSigner{}, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{
		Verdict: review.VerdictPass,
		Findings: []review.Finding{{
			ID:               "note",
			Severity:         review.SeverityLow,
			BlocksCompletion: false,
			Summary:          "advisory note",
		}},
	}}))
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
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), signer, reviewInput(input, &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, reviewer: receipt.Reviewer{Provider: "codex"}}))
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
	if out.Receipt.Body.SnapshotMode != receipt.SnapshotModeWorkingTree || out.Receipt.Body.BaseRef != "" {
		t.Fatalf("snapshot mode = %q base_ref=%q, want working tree without base_ref", out.Receipt.Body.SnapshotMode, out.Receipt.Body.BaseRef)
	}
}

func TestRunStampsBaseDeltaSnapshotModeAndBaseRef(t *testing.T) {
	t.Parallel()

	snapshotter := baseSnapshotter()
	input := baseInput()
	input.BaseRef = "origin/main"
	out, err := Run(context.Background(), snapshotter, passingAcceptance(), &fakeSigner{}, reviewInput(input, &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}))
	if err != nil {
		t.Fatal(err)
	}
	if snapshotter.input.BaseRef != "origin/main" {
		t.Fatalf("snapshot input base ref = %q, want origin/main", snapshotter.input.BaseRef)
	}
	if out.Receipt == nil {
		t.Fatal("expected signed receipt")
	}
	if out.Receipt.Body.SnapshotMode != receipt.SnapshotModeBaseDelta || out.Receipt.Body.BaseRef != "origin/main" {
		t.Fatalf("receipt snapshot mode/base_ref = %q/%q, want base_delta/origin/main", out.Receipt.Body.SnapshotMode, out.Receipt.Body.BaseRef)
	}
}

func TestRunStampsHonestIndependenceReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		input         Input
		reviewer      receipt.Reviewer
		want          []string
		wantDowngrade string
	}{
		{
			name:          "same vendor isolation only",
			input:         baseInput(),
			reviewer:      receipt.Reviewer{Provider: "codex"},
			want:          []string{"isolation_only", "same-vendor", "correlated blind spots"},
			wantDowngrade: receipt.IndependenceDowngradeSameVendor,
		},
		{
			name: "cross vendor still names single party limit",
			input: func() Input {
				input := baseInput()
				input.HostUnderReview.Agent = "codex"
				input.Independence = receipt.Independence{
					Level:      receipt.IndependenceLevelCrossVendor,
					Distinct:   true,
					Downgraded: receipt.IndependenceDowngradeSameVendor,
					Reason:     "legally independent",
				}
				return input
			}(),
			reviewer:      receipt.Reviewer{Provider: "claude"},
			want:          []string{"cross_vendor", "multi-model", "single-party"},
			wantDowngrade: "",
		},
		{
			name: "unknown host is not upgraded",
			input: func() Input {
				input := baseInput()
				input.HostUnderReview.Agent = "unknown"
				input.Independence = receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Distinct: false}
				return input
			}(),
			reviewer:      receipt.Reviewer{Provider: "codex"},
			want:          []string{"host vendor was not detected", "cross_vendor separation is not proven"},
			wantDowngrade: receipt.IndependenceDowngradeUnknownHost,
		},
		{
			name: "cross vendor downgraded when providers are same vendor",
			input: func() Input {
				input := baseInput()
				input.HostUnderReview.Agent = "codex"
				input.Independence = receipt.Independence{Level: receipt.IndependenceLevelCrossVendor, Distinct: true}
				return input
			}(),
			reviewer:      receipt.Reviewer{Provider: "codex"},
			want:          []string{"isolation_only", "same-vendor", "correlated blind spots"},
			wantDowngrade: receipt.IndependenceDowngradeSameVendor,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeSigner{}, reviewInput(tt.input, &fakeReviewer{
				dossier:  review.Dossier{Verdict: review.VerdictPass},
				reviewer: tt.reviewer,
			}))
			if err != nil {
				t.Fatal(err)
			}
			if out.Receipt == nil {
				t.Fatal("expected signed receipt")
			}
			got := out.Receipt.Body.Independence
			if got.Downgraded != tt.wantDowngrade {
				t.Fatalf("downgrade = %q, want %q", got.Downgraded, tt.wantDowngrade)
			}
			reason := got.Reason
			for _, want := range tt.want {
				if !strings.Contains(reason, want) {
					t.Fatalf("independence reason = %q, want fragment %q", reason, want)
				}
			}
		})
	}
}

func TestRunIgnoresAmbientTreeMutationWhenMaterialIsStable(t *testing.T) {
	t.Parallel()

	pre := baseSnapshot()
	post := baseSnapshot()
	post.TreeSHA = "ambient-tree-drift"
	signer := &fakeSigner{}

	out, err := Run(context.Background(), &fakeSnapshotter{snapshots: []Snapshot{pre, post}}, passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass || out.Receipt == nil || signer.called != 1 {
		t.Fatalf("expected ambient drift to pass scoped finalize, got out=%+v signer=%d", out, signer.called)
	}
}

func TestRunFailsClosedOnScopedMaterialMutation(t *testing.T) {
	t.Parallel()

	pre := baseSnapshot()
	post := baseSnapshot()
	post.TreeSHA = "tree-post-mutation"
	post.FileDigests["internal/app/finalize/gate.go"] = "changed-sha"
	signer := &fakeSigner{}
	out, err := Run(context.Background(), &fakeSnapshotter{snapshots: []Snapshot{pre, post}}, passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Receipt != nil || !strings.Contains(out.Reason, "mutated") {
		t.Fatalf("expected fail-closed on scoped material mutation, got %+v", out)
	}
	if signer.called != 0 {
		t.Fatalf("sign ran over mutated material: signer=%d", signer.called)
	}
}

func TestRunFailsClosedOnScopedDeletionMutation(t *testing.T) {
	t.Parallel()

	pre := baseSnapshot()
	pre.Deleted = []string{"internal/app/finalize/removed.go"}
	post := baseSnapshot()
	signer := &fakeSigner{}

	out, err := Run(context.Background(), &fakeSnapshotter{snapshots: []Snapshot{pre, post}}, passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Receipt != nil || !strings.Contains(out.Reason, "mutated") {
		t.Fatalf("expected fail-closed on scoped deletion mutation, got %+v", out)
	}
	if signer.called != 0 {
		t.Fatalf("sign ran over mutated deletion state: signer=%d", signer.called)
	}
}

func TestRunFailsClosedOnScopedIgnoredPathMutation(t *testing.T) {
	t.Parallel()

	pre := baseSnapshot()
	pre.IgnoredUnreviewed = []string{"internal/app/finalize/generated.tmp"}
	post := baseSnapshot()
	signer := &fakeSigner{}

	out, err := Run(context.Background(), &fakeSnapshotter{snapshots: []Snapshot{pre, post}}, passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictFail || out.Receipt != nil || !strings.Contains(out.Reason, "mutated") {
		t.Fatalf("expected fail-closed on scoped ignored-path mutation, got %+v", out)
	}
	if signer.called != 0 {
		t.Fatalf("sign ran over mutated ignored-path state: signer=%d", signer.called)
	}
}

func TestRunStampsReviewedContextProvenance(t *testing.T) {
	t.Parallel()

	prov := []receipt.Provenance{{Kind: "evidence_file", Path: "internal/app/finalize/gate.go", SHA256: "sha", Bytes: 42}}
	out, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), &fakeSigner{}, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, provenance: prov}))
	if err != nil {
		t.Fatal(err)
	}
	if out.Receipt == nil || len(out.Receipt.Body.ReviewedContextProvenance) != 1 || out.Receipt.Body.ReviewedContextProvenance[0].Path != "internal/app/finalize/gate.go" {
		t.Fatalf("reviewed-context provenance not stamped into receipt: %+v", out.Receipt)
	}
}

func TestRunFailsClosedOnUncoveredFileDigest(t *testing.T) {
	t.Parallel()

	// A reviewer that covers nothing (non-nil empty provenance opts out of the test
	// default) must not yield a signed receipt: the snapshot's file digest would be
	// neither reviewed nor declared ignored, so the gate fails closed at mint.
	signer := &fakeSigner{}
	_, err := Run(context.Background(), baseSnapshotter(), passingAcceptance(), signer, reviewInput(baseInput(), &fakeReviewer{dossier: review.Dossier{Verdict: review.VerdictPass}, provenance: []receipt.Provenance{}}))
	if err == nil || !strings.Contains(err.Error(), "not covered") {
		t.Fatalf("gate must fail closed minting an uncovered file digest, got err=%v", err)
	}
	if signer.called != 0 {
		t.Fatalf("an uncovered receipt must never be signed: signer called %d times", signer.called)
	}
}

func baseSnapshotter() *fakeSnapshotter {
	return &fakeSnapshotter{snapshot: baseSnapshot()}
}

func baseSnapshot() Snapshot {
	return Snapshot{
		TreeSHA:           "tree",
		BaseCommit:        "base",
		HeadCommit:        "head",
		FileDigests:       map[string]string{"internal/app/finalize/gate.go": "sha"},
		IgnoredUnreviewed: []string{},
	}
}

func passingAcceptance() *fakeAcceptance {
	return &fakeAcceptance{output: acceptance.EvaluateOutput{
		Results: []acceptance.CriterionResult{{
			ID:           "ac1",
			Command:      "go test ./internal/app/finalize",
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
		Scope:           []string{"internal/app/finalize"},
		SpecFingerprint: "spec",
		HostUnderReview: receipt.HostUnderReview{Agent: "codex"},
		Independence:    receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly},
		Criteria:        []acceptance.Criterion{{ID: "ac1", Command: "go test ./internal/app/finalize", ExpectedKind: "exit_code_zero"}},
		PriorLedgerHead: session.LedgerGenesisHead(),
		MintedAt:        time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
	}
}
