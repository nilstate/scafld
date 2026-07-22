package verify

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

type fakeSnapshotter struct{ snapshot Snapshot }

func (f fakeSnapshotter) Snapshot(context.Context, SnapshotInput) (Snapshot, error) {
	return f.snapshot, nil
}

type recordingSnapshotter struct {
	snapshot Snapshot
	input    SnapshotInput
}

func (r *recordingSnapshotter) Snapshot(_ context.Context, input SnapshotInput) (Snapshot, error) {
	r.input = input
	return r.snapshot, nil
}

type fakeAcceptance struct{ results []AcceptanceResult }

func (f fakeAcceptance) RunAcceptance(context.Context, []receipt.Acceptance) ([]AcceptanceResult, error) {
	return f.results, nil
}

type fakeAncestry struct{ ok bool }

func (f fakeAncestry) IsAncestor(context.Context, string, string) (bool, error) { return f.ok, nil }

type ancestryCall struct {
	ancestor   string
	descendant string
}

type recordingAncestry struct {
	calls []ancestryCall
	ok    map[string]bool
}

func (r *recordingAncestry) IsAncestor(_ context.Context, ancestor, descendant string) (bool, error) {
	r.calls = append(r.calls, ancestryCall{ancestor: ancestor, descendant: descendant})
	if r.ok == nil {
		return true, nil
	}
	if ok, exists := r.ok[ancestor+"->"+descendant]; exists {
		return ok, nil
	}
	return r.ok[descendant], nil
}

type fakeSignature struct{ err error }

func (f fakeSignature) Verify(receipt.Envelope, trust.TrustedKeys) error { return f.err }

func TestRunVerifiesPassingReceipt(t *testing.T) {
	t.Parallel()

	result, err := Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "main", CI: true}, validPorts())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Reason != "verified" {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunReportsInvariantFailures(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		body   func(receipt.Body) receipt.Body
		policy Policy
		ports  func() Ports
		want   string
	}{
		{name: "recorded path missing", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{}}}
			return p
		}, want: "recorded in-scope path missing"},
		{name: "new in-scope path", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha", "new.go": "sha"}}}
			return p
		}, want: "unreviewed in-scope path"},
		{name: "ignored path mismatch", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha"}, Ignored: []string{"AGENTS.md"}}}
			return p
		}, want: "ignored_unreviewed mismatch"},
		{name: "unknown key", ports: func() Ports {
			p := validPorts()
			p.SignatureVerifier = fakeSignature{err: errors.New("unknown key_id")}
			return p
		}, want: "signature verification failed"},
		{name: "revoked key", ports: func() Ports {
			p := validPorts()
			p.SignatureVerifier = fakeSignature{err: errors.New("revoked key")}
			return p
		}, want: "revoked"},
		{name: "duplicate key", ports: func() Ports {
			p := validPorts()
			p.SignatureVerifier = fakeSignature{err: errors.New("duplicate trusted key id")}
			return p
		}, want: "duplicate"},
		{name: "mismatched key id", ports: func() Ports {
			p := validPorts()
			p.SignatureVerifier = fakeSignature{err: errors.New("key_id mismatch")}
			return p
		}, want: "key_id mismatch"},
		{name: "non-pass verdict", body: func(b receipt.Body) receipt.Body {
			b.Verdict = "fail"
			return b
		}, want: "verdict"},
		{name: "nonzero open blockers", body: func(b receipt.Body) receipt.Body {
			b.OpenBlockers = []receipt.Blocker{{ID: "b"}}
			return b
		}, want: "open_blockers"},
		{name: "dirty mutation_guard", body: func(b receipt.Body) receipt.Body {
			b.MutationGuard.Status = "dirty"
			return b
		}, want: "mutation_guard"},
		{name: "same vendor below cross vendor", body: func(b receipt.Body) receipt.Body {
			b.Independence = receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor}
			return b
		}, policy: Policy{TargetCommit: "main", MinIndependence: receipt.IndependenceLevelCrossVendor}, want: "independence"},
		{name: "forged downgrade", body: func(b receipt.Body) receipt.Body {
			b.Independence = receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly}
			return b
		}, want: "downgrade"},
		{name: "unknown snapshot mode", body: func(b receipt.Body) receipt.Body {
			b.SnapshotMode = "future_mode"
			return b
		}, want: "snapshot_mode"},
		{name: "forged cross vendor stamp", body: func(b receipt.Body) receipt.Body {
			b.Reviewer.Provider = "claude"
			b.HostUnderReview.Agent = "claude"
			b.Independence = receipt.Independence{Level: receipt.IndependenceLevelCrossVendor, Distinct: true}
			return b
		}, policy: Policy{TargetCommit: "main", MinIndependence: receipt.IndependenceLevelCrossVendor}, want: "does not match"},
		{name: "unreviewed in-scope digest", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha", "b.go": "sha"}}}
			return p
		}, want: "unreviewed in-scope path"},
		{name: "base commit mismatch", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "other", FileDigests: map[string]string{"a.go": "sha"}}}
			return p
		}, want: "base_commit mismatch"},
		{name: "non ancestor base commit", ports: func() Ports {
			p := validPorts()
			p.AncestryChecker = fakeAncestry{ok: false}
			return p
		}, policy: Policy{TargetCommit: "main"}, want: "ancestor"},
		{name: "command provider", body: func(b receipt.Body) receipt.Body {
			b.Reviewer.Provider = "command"
			return b
		}, want: "command/human"},
		{name: "human provider", body: func(b receipt.Body) receipt.Body {
			b.Reviewer.Provider = "human"
			return b
		}, want: "command/human"},
		{name: "acceptance mismatch", ports: func() Ports {
			p := validPorts()
			p.AcceptanceRunner = fakeAcceptance{results: []AcceptanceResult{{ID: "ac1", Status: "fail", ExitCode: 1}}}
			return p
		}, want: "acceptance mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			envelope := validEnvelope()
			if tt.body != nil {
				envelope.Body = tt.body(envelope.Body)
			}
			policy := tt.policy
			if policy.TargetCommit == "" {
				policy.TargetCommit = "main"
			}
			ports := validPorts()
			if tt.ports != nil {
				ports = tt.ports()
			}
			result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, policy, ports)
			if err != nil {
				t.Fatal(err)
			}
			if result.Passed || !strings.Contains(result.Reason, tt.want) {
				t.Fatalf("result = %+v, want failure containing %q", result, tt.want)
			}
		})
	}
}

func TestRunUsesSignedBaseCommitInsteadOfTargetAsSnapshotBase(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.Body.SnapshotMode = receipt.SnapshotModeBaseDelta
	envelope.Body.BaseRef = "origin/main"
	snapshotter := &recordingSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha"}}}
	ports := validPorts()
	ports.Snapshotter = snapshotter
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "main"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("result = %+v", result)
	}
	if snapshotter.input.SnapshotMode != receipt.SnapshotModeBaseDelta || snapshotter.input.BaseRef != "base" {
		t.Fatalf("snapshot input = %+v, want signed base commit; target is ancestry-only", snapshotter.input)
	}
}

func TestRunAllowsOutOfScopeTreeChangesWhenScopedStateMatches(t *testing.T) {
	envelope := validEnvelope()
	snapshotter := &recordingSnapshotter{snapshot: Snapshot{
		TreeSHA:     "different-full-tree",
		BaseCommit:  "base",
		FileDigests: map[string]string{"a.go": "sha"},
	}}
	ports := validPorts()
	ports.Snapshotter = snapshotter
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("out-of-scope tree change should not invalidate scoped receipt: %+v", result)
	}
}

func TestRunUsesBaseCommitForBaseDeltaWithoutOriginalBaseRef(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.Body.SnapshotMode = receipt.SnapshotModeBaseDelta
	envelope.Body.BaseRef = ""
	snapshotter := &recordingSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha"}}}
	ports := validPorts()
	ports.Snapshotter = snapshotter
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("result = %+v", result)
	}
	if snapshotter.input.SnapshotMode != receipt.SnapshotModeBaseDelta || snapshotter.input.BaseRef != "base" {
		t.Fatalf("snapshot input = %+v, want signed base commit fallback", snapshotter.input)
	}
}

func TestRunPassesMaterialRefToSnapshotter(t *testing.T) {
	t.Parallel()

	snapshotter := &recordingSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha"}}}
	ports := validPorts()
	ports.Snapshotter = snapshotter
	result, err := Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "base", MaterialRef: "head"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("result = %+v", result)
	}
	if snapshotter.input.TargetRef != "head" {
		t.Fatalf("snapshot input target = %q, want head", snapshotter.input.TargetRef)
	}
}

func TestRunMaterialOnlySkipsAcceptance(t *testing.T) {
	t.Parallel()

	ports := validPorts()
	ports.AcceptanceRunner = nil
	result, err := Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "base", MaterialOnly: true}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || result.Reason != "material verified" {
		t.Fatalf("result = %+v, want material-only pass", result)
	}
}

func TestRunChecksTargetAndMaterialRefAncestry(t *testing.T) {
	t.Parallel()

	ancestry := &recordingAncestry{ok: map[string]bool{"protected": true, "head": true}}
	ports := validPorts()
	ports.AncestryChecker = ancestry
	result, err := Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "protected", MaterialRef: "head"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("result = %+v", result)
	}
	if len(ancestry.calls) != 3 || ancestry.calls[0].descendant != "protected" || ancestry.calls[1].descendant != "head" || ancestry.calls[2] != (ancestryCall{ancestor: "protected", descendant: "head"}) {
		t.Fatalf("ancestry calls = %+v, want receipt base checks plus protected target against material ref", ancestry.calls)
	}

	ancestry = &recordingAncestry{ok: map[string]bool{"protected": false, "head": true}}
	ports = validPorts()
	ports.AncestryChecker = ancestry
	result, err = Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "protected", MaterialRef: "head"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "target") {
		t.Fatalf("result = %+v, want target ancestry failure", result)
	}
	if len(ancestry.calls) != 1 || ancestry.calls[0].descendant != "protected" {
		t.Fatalf("target ancestry boundary was not checked first: %+v", ancestry.calls)
	}

	ancestry = &recordingAncestry{ok: map[string]bool{"protected": true, "head": true, "protected->head": false}}
	ports = validPorts()
	ports.AncestryChecker = ancestry
	result, err = Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "protected", MaterialRef: "head"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "target is not ancestor") {
		t.Fatalf("result = %+v, want target/material divergence failure", result)
	}
}

func TestRunAcceptsGenuineCrossVendor(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.Body.Reviewer.Provider = "claude"
	envelope.Body.HostUnderReview.Agent = "codex"
	envelope.Body.Independence = receipt.Independence{Level: receipt.IndependenceLevelCrossVendor, Distinct: true}
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "main", MinIndependence: receipt.IndependenceLevelCrossVendor}, validPorts())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed {
		t.Fatalf("genuine cross-vendor receipt must pass cross_vendor policy: %+v", result)
	}
}

func TestRunTargetRequiredInCI(t *testing.T) {
	t.Parallel()

	result, err := Run(context.Background(), validEnvelope(), trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{CI: true}, validPorts())
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "missing target") {
		t.Fatalf("result = %+v", result)
	}
}

func TestVerifyRejectsDigestValueMismatch(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	ports := validPorts()
	// Same tree_sha and the same in-scope path, but the recomputed digest value
	// differs from what the receipt recorded; verify must reject it.
	ports.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: envelope.Body.TreeSHA, BaseCommit: envelope.Body.BaseCommit, FileDigests: map[string]string{"a.go": "different-sha"}}}
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "main"}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "file digest mismatch") {
		t.Fatalf("result = %+v, want file digest mismatch failure", result)
	}
}

func validPorts() Ports {
	return Ports{
		Snapshotter:       fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", BaseCommit: "base", FileDigests: map[string]string{"a.go": "sha"}}},
		AcceptanceRunner:  fakeAcceptance{results: []AcceptanceResult{{ID: "ac1", Status: "pass", ExitCode: 0}}},
		AncestryChecker:   fakeAncestry{ok: true},
		SignatureVerifier: fakeSignature{},
	}
}

func validEnvelope() receipt.Envelope {
	return receipt.Envelope{
		Body: receipt.Body{
			SchemaVersion:             receipt.SchemaVersion,
			TaskID:                    "task",
			SessionID:                 "session",
			Verdict:                   "pass",
			SnapshotMode:              receipt.SnapshotModeWorkingTree,
			BaseCommit:                "base",
			HeadCommit:                "head",
			Scope:                     []string{"."},
			TreeSHA:                   "tree",
			FileDigests:               map[string]string{"a.go": "sha"},
			IgnoredUnreviewed:         []string{},
			ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "sha"}},
			Reviewer:                  receipt.Reviewer{Provider: "codex"},
			HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
			Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
			SpecFingerprint:           "spec",
			AcceptanceDeclared:        true,
			Acceptance:                []receipt.Acceptance{{ID: "ac1", Status: "pass", ExitCode: 0}},
			OpenBlockers:              []receipt.Blocker{},
			MutationGuard:             receipt.MutationGuard{Status: "clean"},
			LedgerHead:                "ledger",
			MintedAt:                  "2026-06-03T00:00:00Z",
		},
		Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"},
	}
}
