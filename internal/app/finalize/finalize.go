// Package finalize owns the host-facing accountability finalization use case.
package finalize

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/app/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
)

// Snapshotter computes the commit-free tree facts reviewed by finalization.
type Snapshotter interface {
	Snapshot(context.Context, SnapshotInput) (Snapshot, error)
}

// AcceptanceRunner evaluates declared acceptance criteria.
type AcceptanceRunner interface {
	Evaluate(context.Context, acceptance.EvaluateInput) (acceptance.EvaluateOutput, error)
}

// Reviewer runs the isolated adversarial review over the snapshot tree and
// returns the dossier, the provenance of the exact bytes it reviewed, and the
// runtime facts of the reviewer that actually ran.
type Reviewer interface {
	Review(context.Context, ReviewInput) (ReviewResult, error)
}

// ReviewResult is the reviewer port output stamped into the signed receipt. The
// Reviewer facts come from the invocation that actually ran, never a separate
// pre-acceptance probe, so the receipt cannot attest a stale reviewer binary.
// Ignored lists in-scope paths the reviewer deliberately did not read (submodule
// gitlinks, governed instruction/config files); they are signed in file_digests
// but recorded as ignored_unreviewed so the receipt never implies their review.
type ReviewResult struct {
	Dossier    review.Dossier
	Provenance []receipt.Provenance
	Ignored    []string
	Reviewer   receipt.Reviewer
}

// Signer signs the canonical receipt body.
type Signer interface {
	Sign(receipt.Body) (receipt.DetachedSignature, error)
}

// SnapshotInput scopes the tree fingerprint.
type SnapshotInput struct {
	Scope   []string
	BaseRef string
}

// Snapshot captures the deterministic tree facts included in the receipt.
type Snapshot struct {
	TreeSHA           string
	BaseCommit        string
	HeadCommit        string
	FileDigests       map[string]string
	IgnoredUnreviewed []string
	Deleted           []string
}

// ReviewInput is the app-level review request.
type ReviewInput struct {
	TaskID     string
	TreeSHA    string
	Scope      []string
	Deleted    []string
	Depth      string
	DiffScoped bool
}

// Input configures one gate run.
type Input struct {
	TaskID           string
	SessionID        string
	Scope            []string
	BaseRef          string
	ReviewerProvider string
	SpecFingerprint  string
	HostUnderReview  receipt.HostUnderReview
	Independence     receipt.Independence
	Criteria         []acceptance.Criterion
	WorkDir          string
	Env              []string
	Timeout          time.Duration
	IdleTimeout      time.Duration
	PriorLedgerHead  string
	MintedAt         time.Time
}

// Output is the structured gate result.
type Output struct {
	Verdict      string
	Acceptance   acceptance.EvaluateOutput
	Findings     []review.Finding
	Receipt      *receipt.Envelope
	Independence receipt.Independence
	Reason       string
}

// Run executes snapshot, acceptance, review, calibration, and receipt minting.
func Run(ctx context.Context, snapshotter Snapshotter, acceptanceRunner AcceptanceRunner, reviewer Reviewer, signer Signer, input Input) (Output, error) {
	if snapshotter == nil {
		return Output{}, errors.New("snapshotter is required")
	}
	if acceptanceRunner == nil {
		return Output{}, errors.New("acceptance runner is required")
	}
	if reviewer == nil {
		return Output{}, errors.New("reviewer is required")
	}
	if signer == nil {
		return Output{}, errors.New("signer is required")
	}
	if len(normalizedScope(input.Scope)) == 0 {
		return Output{}, errors.New("gate scope is empty; refusing to mint unscoped receipt")
	}
	selectedIndependence := completeIndependence(input.Independence, input.ReviewerProvider, input.HostUnderReview.Agent)
	snap, err := snapshotter.Snapshot(ctx, SnapshotInput{Scope: input.Scope, BaseRef: input.BaseRef})
	if err != nil {
		return Output{}, fmt.Errorf("snapshot: %w", err)
	}
	accepted, err := acceptanceRunner.Evaluate(ctx, acceptance.EvaluateInput{
		Criteria:    input.Criteria,
		WorkDir:     input.WorkDir,
		Env:         input.Env,
		Timeout:     input.Timeout,
		IdleTimeout: input.IdleTimeout,
		FailFast:    true,
	})
	if err != nil {
		return Output{}, fmt.Errorf("acceptance: %w", err)
	}
	if !accepted.Passed {
		return Output{Verdict: review.VerdictFail, Acceptance: accepted, Independence: selectedIndependence, Reason: "acceptance failed before review"}, nil
	}
	// Mutation guard: acceptance may have rewritten scoped files. Re-snapshot and
	// fail closed if the tree drifted, so the reviewer and the signed receipt
	// describe the exact same bytes.
	post, err := snapshotter.Snapshot(ctx, SnapshotInput{Scope: input.Scope, BaseRef: input.BaseRef})
	if err != nil {
		return Output{}, fmt.Errorf("mutation snapshot: %w", err)
	}
	if post.TreeSHA != snap.TreeSHA {
		return Output{Verdict: review.VerdictFail, Acceptance: accepted, Independence: selectedIndependence, Reason: "workspace mutated during acceptance; gate failed closed"}, nil
	}
	result, err := reviewer.Review(ctx, ReviewInput{
		TaskID:     input.TaskID,
		TreeSHA:    snap.TreeSHA,
		Scope:      append([]string(nil), input.Scope...),
		Deleted:    append([]string(nil), snap.Deleted...),
		Depth:      "light",
		DiffScoped: true,
	})
	if err != nil {
		return Output{}, fmt.Errorf("review: %w", err)
	}
	findings := downgradeUnsubstantiatedBlockers(result.Dossier.Findings)
	blockers := receiptBlockers(findings)
	reviewedIndependence := completeIndependence(input.Independence, result.Reviewer.Provider, input.HostUnderReview.Agent)
	if len(blockers) > 0 {
		return Output{Verdict: review.VerdictFail, Acceptance: accepted, Findings: findings, Independence: reviewedIndependence, Reason: "review blockers remain"}, nil
	}
	body := receiptBody(input, snap, accepted, blockers, result.Provenance, result.Ignored, result.Reviewer)
	// Fail closed before signing if any signed file digest is neither reviewed nor
	// declared ignored, so the gate can never mint a receipt that implies review of
	// bytes the independent reviewer never saw.
	if err := receipt.ValidateReviewCoverage(body); err != nil {
		return Output{}, fmt.Errorf("mint receipt: %w", err)
	}
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		return Output{}, err
	}
	body.LedgerHead = session.NextLedgerHead(input.PriorLedgerHead, digest)
	signature, err := signer.Sign(body)
	if err != nil {
		return Output{}, fmt.Errorf("sign receipt: %w", err)
	}
	envelope := receipt.Envelope{Body: body, Signature: signature}
	return Output{Verdict: review.VerdictPass, Acceptance: accepted, Findings: findings, Receipt: &envelope, Independence: body.Independence}, nil
}

func normalizedScope(scope []string) []string {
	var out []string
	for _, item := range scope {
		if strings.TrimSpace(item) != "" {
			out = append(out, item)
		}
	}
	return out
}

// downgradeUnsubstantiatedBlockers turns a claimed blocker into advisory when
// either Location or Validation is missing. That downgrade prevents advisory
// findings from gating completion while preserving the finding for the receipt.
func downgradeUnsubstantiatedBlockers(findings []review.Finding) []review.Finding {
	out := append([]review.Finding(nil), findings...)
	for i := range out {
		if !out[i].BlocksCompletion {
			continue
		}
		if out[i].Location == nil || strings.TrimSpace(out[i].Location.Path) == "" || strings.TrimSpace(out[i].Validation) == "" {
			out[i].BlocksCompletion = false
			if out[i].Confidence == "" {
				out[i].Confidence = review.ConfidenceLow
			}
			if !strings.Contains(strings.ToLower(out[i].Summary), "advisory") {
				out[i].Summary = strings.TrimSpace(out[i].Summary + " (downgraded to advisory: missing location or validation)")
			}
		}
	}
	return out
}

func receiptBody(input Input, snap Snapshot, accepted acceptance.EvaluateOutput, blockers []receipt.Blocker, provenance []receipt.Provenance, ignored []string, reviewer receipt.Reviewer) receipt.Body {
	mintedAt := input.MintedAt
	if mintedAt.IsZero() {
		mintedAt = time.Now().UTC()
	}
	if provenance == nil {
		provenance = []receipt.Provenance{}
	}
	independence := completeIndependence(input.Independence, reviewer.Provider, input.HostUnderReview.Agent)
	mode := snapshotMode(input.BaseRef)
	baseRef := ""
	if mode == receipt.SnapshotModeBaseDelta {
		baseRef = strings.TrimSpace(input.BaseRef)
	}
	return receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    input.TaskID,
		SessionID:                 input.SessionID,
		Verdict:                   review.VerdictPass,
		SnapshotMode:              mode,
		BaseRef:                   baseRef,
		BaseCommit:                snap.BaseCommit,
		HeadCommit:                snap.HeadCommit,
		Scope:                     cloneSlice(input.Scope),
		TreeSHA:                   snap.TreeSHA,
		FileDigests:               cloneMap(snap.FileDigests),
		IgnoredUnreviewed:         mergeUnique(snap.IgnoredUnreviewed, ignored),
		ReviewedContextProvenance: provenance,
		Reviewer:                  reviewer,
		HostUnderReview:           input.HostUnderReview,
		Independence:              independence,
		SpecFingerprint:           input.SpecFingerprint,
		AcceptanceDeclared:        len(input.Criteria) > 0,
		Acceptance:                receiptAcceptance(accepted.Results),
		OpenBlockers:              cloneBlockers(blockers),
		MutationGuard:             receipt.MutationGuard{Status: "clean", Scope: cloneSlice(input.Scope)},
		MintedAt:                  mintedAt.UTC().Format(time.RFC3339),
	}
}

func snapshotMode(baseRef string) string {
	if strings.TrimSpace(baseRef) != "" {
		return receipt.SnapshotModeBaseDelta
	}
	return receipt.SnapshotModeWorkingTree
}

func completeIndependence(ind receipt.Independence, reviewerProvider string, hostAgent string) receipt.Independence {
	// Fully re-derive the signed level and distinctness from the reviewer that
	// actually ran versus the detected host, never trusting the passed-in level.
	// cross_vendor is stamped only when separation is genuinely proven, so neither a
	// forged cross_vendor input nor a stale isolation_only input can survive.
	if crossVendorProven(reviewerProvider, hostAgent) {
		ind.Level = receipt.IndependenceLevelCrossVendor
		ind.Distinct = true
	} else {
		ind.Level = receipt.IndependenceLevelIsolationOnly
		ind.Distinct = false
	}
	ind.Downgraded = independenceDowngrade(ind, reviewerProvider, hostAgent)
	ind.Reason = independenceReason(ind, reviewerProvider, hostAgent)
	return ind
}

// crossVendorProven reports whether the reviewer and host are both known and from
// different vendors, the only case in which cross_vendor independence is real.
func crossVendorProven(reviewerProvider, hostAgent string) bool {
	reviewerProvider = strings.TrimSpace(reviewerProvider)
	hostAgent = strings.TrimSpace(hostAgent)
	if reviewerProvider == "" || strings.EqualFold(reviewerProvider, "unknown") {
		return false
	}
	if hostAgent == "" || strings.EqualFold(hostAgent, "unknown") {
		return false
	}
	return !strings.EqualFold(reviewerProvider, hostAgent)
}

func independenceDowngrade(ind receipt.Independence, reviewerProvider string, hostAgent string) string {
	if ind.Level == receipt.IndependenceLevelCrossVendor {
		return ""
	}
	reviewerProvider = strings.TrimSpace(reviewerProvider)
	hostAgent = strings.TrimSpace(hostAgent)
	if reviewerProvider == "" || strings.EqualFold(reviewerProvider, "unknown") {
		return receipt.IndependenceDowngradeUnknownReviewer
	}
	if hostAgent == "" || strings.EqualFold(hostAgent, "unknown") {
		return receipt.IndependenceDowngradeUnknownHost
	}
	if strings.EqualFold(reviewerProvider, hostAgent) {
		return receipt.IndependenceDowngradeSameVendor
	}
	return ""
}

func independenceReason(ind receipt.Independence, reviewerProvider string, hostAgent string) string {
	reviewerProvider = strings.TrimSpace(reviewerProvider)
	hostAgent = strings.TrimSpace(hostAgent)
	switch ind.Level {
	case receipt.IndependenceLevelCrossVendor:
		return fmt.Sprintf("cross_vendor: reviewer %q and host %q are different model vendors; this is multi-model review that reduces correlated blind spots but remains single-party local tooling", reviewerProvider, hostAgent)
	case receipt.IndependenceLevelIsolationOnly:
		if hostAgent == "" || strings.EqualFold(hostAgent, "unknown") {
			return fmt.Sprintf("isolation_only: reviewer %q ran in an isolated path, but the host vendor was not detected so cross_vendor separation is not proven", reviewerProvider)
		}
		if strings.EqualFold(reviewerProvider, hostAgent) {
			return fmt.Sprintf("isolation_only: reviewer %q matches host %q; the review is isolated but same-vendor, so correlated blind spots remain possible", reviewerProvider, hostAgent)
		}
		return fmt.Sprintf("isolation_only: reviewer %q ran separately from host %q, but cross_vendor separation is not proven", reviewerProvider, hostAgent)
	default:
		return fmt.Sprintf("unrecognized independence level %q; verify policy remains authoritative", ind.Level)
	}
}

func receiptAcceptance(results []acceptance.CriterionResult) []receipt.Acceptance {
	out := make([]receipt.Acceptance, 0, len(results))
	for _, result := range results {
		out = append(out, receipt.Acceptance{
			ID:           result.ID,
			Command:      result.Command,
			ExpectedKind: result.ExpectedKind,
			Status:       result.Status,
			Reason:       result.Reason,
			ExitCode:     result.ExitCode,
			OutputSHA256: result.StdoutDigest,
			Diagnostic:   result.DiagnosticPath,
		})
	}
	return out
}

func receiptBlockers(findings []review.Finding) []receipt.Blocker {
	var blockers []receipt.Blocker
	for _, finding := range findings {
		if !review.BlocksCompletion(finding) {
			continue
		}
		location := ""
		if finding.Location != nil {
			location = finding.Location.Path
			if finding.Location.Line > 0 {
				location = fmt.Sprintf("%s:%d", location, finding.Location.Line)
			}
		}
		blockers = append(blockers, receipt.Blocker{
			ID:         finding.ID,
			Severity:   string(finding.Severity),
			Location:   location,
			Summary:    finding.Summary,
			Validation: finding.Validation,
		})
	}
	return blockers
}

func cloneMap(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneSlice(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

// mergeUnique returns the deterministic union of two path lists, dropping empties
// and duplicates, so ignored_unreviewed is canonical regardless of input order.
func mergeUnique(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, list := range [][]string{a, b} {
		for _, value := range list {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func cloneBlockers(values []receipt.Blocker) []receipt.Blocker {
	if len(values) == 0 {
		return []receipt.Blocker{}
	}
	return append([]receipt.Blocker(nil), values...)
}
