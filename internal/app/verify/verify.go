// Package verify owns the CI-side receipt verification use case.
package verify

import (
	"context"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

// Snapshotter recomputes the checked-out tree fingerprint.
type Snapshotter interface {
	Snapshot(context.Context, SnapshotInput) (Snapshot, error)
}

// AcceptanceRunner re-runs recorded acceptance commands.
type AcceptanceRunner interface {
	RunAcceptance(context.Context, []receipt.Acceptance) ([]AcceptanceResult, error)
}

// AncestryChecker verifies base_commit ancestry against the explicit target.
type AncestryChecker interface {
	IsAncestor(context.Context, string, string) (bool, error)
}

// SignatureVerifier verifies detached receipt signatures.
type SignatureVerifier interface {
	Verify(receipt.Envelope, trust.TrustedKeys) error
}

// Ports groups the narrow verification ports.
type Ports struct {
	Snapshotter       Snapshotter
	AcceptanceRunner  AcceptanceRunner
	AncestryChecker   AncestryChecker
	SignatureVerifier SignatureVerifier
}

// SnapshotInput scopes verification fingerprinting.
type SnapshotInput struct {
	Scope        []string
	SnapshotMode string
	BaseRef      string
}

// Snapshot is the recomputed tree state.
type Snapshot struct {
	TreeSHA     string
	BaseCommit  string
	FileDigests map[string]string
	Ignored     []string
}

// AcceptanceResult is one observed acceptance rerun result.
type AcceptanceResult struct {
	ID       string
	Status   string
	ExitCode int
}

// Policy controls pure verify invariants.
type Policy struct {
	TargetCommit    string
	CI              bool
	MinIndependence string
}

// Result is the structured verification verdict.
type Result struct {
	Passed bool
	Reason string
}

// Run verifies every receipt invariant against current reality.
func Run(ctx context.Context, envelope receipt.Envelope, trusted trust.TrustedKeys, policy Policy, ports Ports) (Result, error) {
	if policy.CI && strings.TrimSpace(policy.TargetCommit) == "" {
		return fail("missing target in CI policy"), nil
	}
	if err := requirePorts(ports); err != nil {
		return Result{}, err
	}
	if err := receipt.ValidateEnvelope(envelope); err != nil {
		return fail("invalid receipt: " + err.Error()), nil
	}
	if err := ports.SignatureVerifier.Verify(envelope, trusted); err != nil {
		return fail("signature verification failed: " + err.Error()), nil
	}
	body := envelope.Body
	if body.Verdict != review.VerdictPass {
		return fail("verdict is not pass"), nil
	}
	if len(body.OpenBlockers) != 0 {
		return fail("open_blockers is nonzero"), nil
	}
	if body.MutationGuard.Status != "clean" {
		return fail("mutation_guard is dirty"), nil
	}
	if providerRejected(body.Reviewer.Provider) {
		return fail("command/human providers are rejected"), nil
	}
	if ok, reason := meetsIndependence(body.Independence, body.Reviewer.Provider, body.HostUnderReview.Agent, policy.MinIndependence); !ok {
		return fail(reason), nil
	}
	baseRef, ok := verifySnapshotBaseRef(body, policy.TargetCommit)
	if !ok {
		return fail("unknown snapshot_mode"), nil
	}
	snapshot, err := ports.Snapshotter.Snapshot(ctx, SnapshotInput{Scope: body.Scope, SnapshotMode: body.SnapshotMode, BaseRef: baseRef})
	if err != nil {
		return Result{}, err
	}
	if snapshot.TreeSHA != body.TreeSHA {
		return fail("tree mismatch"), nil
	}
	if strings.TrimSpace(snapshot.BaseCommit) != strings.TrimSpace(body.BaseCommit) {
		return fail("base_commit mismatch"), nil
	}
	if missing := missingCoverage(snapshot.FileDigests, body.FileDigests); len(missing) > 0 {
		return fail("missing in-scope digest: " + strings.Join(missing, ",")), nil
	}
	if strings.TrimSpace(policy.TargetCommit) != "" {
		ok, err := ports.AncestryChecker.IsAncestor(ctx, body.BaseCommit, policy.TargetCommit)
		if err != nil {
			return Result{}, err
		}
		if !ok {
			return fail("base_commit is not ancestor of target"), nil
		}
	}
	observed, err := ports.AcceptanceRunner.RunAcceptance(ctx, body.Acceptance)
	if err != nil {
		return Result{}, err
	}
	if reason := acceptanceMismatch(body.Acceptance, observed); reason != "" {
		return fail(reason), nil
	}
	return Result{Passed: true, Reason: "verified"}, nil
}

func verifySnapshotBaseRef(body receipt.Body, targetCommit string) (string, bool) {
	switch body.SnapshotMode {
	case receipt.SnapshotModeWorkingTree:
		return "", true
	case receipt.SnapshotModeBaseDelta:
		if ref := strings.TrimSpace(targetCommit); ref != "" {
			return ref, true
		}
		if ref := strings.TrimSpace(body.BaseRef); ref != "" {
			return ref, true
		}
		return strings.TrimSpace(body.BaseCommit), true
	default:
		return "", false
	}
}

func requirePorts(ports Ports) error {
	switch {
	case ports.Snapshotter == nil:
		return fmt.Errorf("snapshotter is required")
	case ports.AcceptanceRunner == nil:
		return fmt.Errorf("acceptance runner is required")
	case ports.AncestryChecker == nil:
		return fmt.Errorf("ancestry checker is required")
	case ports.SignatureVerifier == nil:
		return fmt.Errorf("signature verifier is required")
	default:
		return nil
	}
}

func providerRejected(provider string) bool {
	switch strings.TrimSpace(provider) {
	case "command", "human":
		return true
	default:
		return !review.ValidCompletionProvider(provider)
	}
}

// meetsIndependence re-derives the independence level from the signed reviewer
// and host vendors instead of trusting the self-reported independence fields, so
// a host cannot stamp a same-vendor review as cross_vendor. The signed fields
// must agree with the re-derivation, and a cross_vendor policy requires two
// known, distinct vendors.
func meetsIndependence(ind receipt.Independence, reviewer, host, minimum string) (bool, string) {
	reviewerVendor := normalizeVendor(reviewer)
	hostVendor := normalizeVendor(host)
	distinct := reviewerVendor != "" && hostVendor != "" && reviewerVendor != hostVendor
	level := receipt.IndependenceLevelIsolationOnly
	if distinct {
		level = receipt.IndependenceLevelCrossVendor
	}
	if ind.Level != level || ind.Distinct != distinct {
		return false, "independence claim does not match reviewer and host vendors"
	}
	if ind.Downgraded != expectedIndependenceDowngrade(level, reviewerVendor, hostVendor) {
		return false, "independence downgrade does not match reviewer and host vendors"
	}
	switch strings.TrimSpace(minimum) {
	case "", receipt.IndependenceLevelIsolationOnly:
		return true, ""
	case receipt.IndependenceLevelCrossVendor:
		if level == receipt.IndependenceLevelCrossVendor {
			return true, ""
		}
		return false, "below minimum independence"
	default:
		return false, "unknown min_independence policy"
	}
}

func expectedIndependenceDowngrade(level string, reviewerVendor string, hostVendor string) string {
	if level == receipt.IndependenceLevelCrossVendor {
		return ""
	}
	if reviewerVendor == "" {
		return receipt.IndependenceDowngradeUnknownReviewer
	}
	if hostVendor == "" {
		return receipt.IndependenceDowngradeUnknownHost
	}
	if reviewerVendor == hostVendor {
		return receipt.IndependenceDowngradeSameVendor
	}
	return ""
}

// normalizeVendor lowercases a vendor and folds the "unknown" sentinel (recorded
// when the gate cannot detect the host from environment markers) to the empty
// vendor, so an undetected host is never treated as distinct from the reviewer.
func normalizeVendor(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "unknown" {
		return ""
	}
	return value
}

func missingCoverage(snapshot map[string]string, recorded map[string]string) []string {
	var missing []string
	for path := range snapshot {
		if _, ok := recorded[path]; !ok {
			missing = append(missing, path)
		}
	}
	return missing
}

func acceptanceMismatch(recorded []receipt.Acceptance, observed []AcceptanceResult) string {
	byID := map[string]AcceptanceResult{}
	for _, result := range observed {
		byID[result.ID] = result
	}
	for _, want := range recorded {
		got, ok := byID[want.ID]
		if !ok {
			return "acceptance missing observed result: " + want.ID
		}
		if got.Status != want.Status || got.ExitCode != want.ExitCode {
			return "acceptance mismatch: " + want.ID
		}
	}
	return ""
}

func fail(reason string) Result {
	return Result{Passed: false, Reason: reason}
}
