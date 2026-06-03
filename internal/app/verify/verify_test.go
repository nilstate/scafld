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

type fakeAcceptance struct{ results []AcceptanceResult }

func (f fakeAcceptance) RunAcceptance(context.Context, []receipt.Acceptance) ([]AcceptanceResult, error) {
	return f.results, nil
}

type fakeAncestry struct{ ok bool }

func (f fakeAncestry) IsAncestor(context.Context, string, string) (bool, error) { return f.ok, nil }

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
		{name: "tree mismatch", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "different", FileDigests: map[string]string{"a.go": "sha"}}}
			return p
		}, want: "tree mismatch"},
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
			b.Independence = receipt.Independence{Level: "isolation_only"}
			return b
		}, policy: Policy{TargetCommit: "main", MinIndependence: "cross_vendor"}, want: "independence"},
		{name: "forged cross vendor stamp", body: func(b receipt.Body) receipt.Body {
			b.Reviewer.Provider = "claude"
			b.HostUnderReview.Agent = "claude"
			b.Independence = receipt.Independence{Level: "cross_vendor", Distinct: true}
			return b
		}, policy: Policy{TargetCommit: "main", MinIndependence: "cross_vendor"}, want: "does not match"},
		{name: "missing in-scope digest", ports: func() Ports {
			p := validPorts()
			p.Snapshotter = fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", FileDigests: map[string]string{"a.go": "sha", "b.go": "sha"}}}
			return p
		}, want: "missing in-scope digest"},
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

func TestRunAcceptsGenuineCrossVendor(t *testing.T) {
	t.Parallel()

	envelope := validEnvelope()
	envelope.Body.Reviewer.Provider = "claude"
	envelope.Body.HostUnderReview.Agent = "codex"
	envelope.Body.Independence = receipt.Independence{Level: "cross_vendor", Distinct: true}
	result, err := Run(context.Background(), envelope, trust.TrustedKeys{Version: trust.TrustedKeysVersion}, Policy{TargetCommit: "main", MinIndependence: "cross_vendor"}, validPorts())
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

func validPorts() Ports {
	return Ports{
		Snapshotter:       fakeSnapshotter{snapshot: Snapshot{TreeSHA: "tree", FileDigests: map[string]string{"a.go": "sha"}}},
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
			BaseCommit:                "base",
			HeadCommit:                "head",
			Scope:                     []string{"."},
			TreeSHA:                   "tree",
			FileDigests:               map[string]string{"a.go": "sha"},
			IgnoredUnreviewed:         []string{},
			ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "sha"}},
			Reviewer:                  receipt.Reviewer{Provider: "codex"},
			HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
			Independence:              receipt.Independence{Level: "isolation_only"},
			SpecFingerprint:           "spec",
			Acceptance:                []receipt.Acceptance{{ID: "ac1", Status: "pass", ExitCode: 0}},
			OpenBlockers:              []receipt.Blocker{},
			MutationGuard:             receipt.MutationGuard{Status: "clean"},
			LedgerHead:                "ledger",
			MintedAt:                  "2026-06-03T00:00:00Z",
		},
		Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"},
	}
}
