// Package completion evaluates the review authority that allows a task to close.
package completion

import (
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
)

// Authority describes the review evidence that authorizes completion.
type Authority struct {
	Found         bool
	Completed     bool
	Valid         bool
	HumanReviewed bool
	Reason        string
	Actual        string
	Evidence      []string
	ReviewEntry   session.Entry
	ReceiptEntry  session.Entry
	CompleteEntry session.Entry
	Dossier       corereview.Dossier
	HasDossier    bool
	Receipt       receipt.Envelope
	HasReceipt    bool
}

// Status returns valid or invalid for display surfaces.
func (a Authority) Status() string {
	if a.Valid {
		return "valid"
	}
	return "invalid"
}

// Kind returns a stable label for the completion authority.
func (a Authority) Kind() string {
	switch {
	case !a.Found || !a.Valid:
		return "invalid"
	case a.HasReceipt:
		return "receipt"
	case a.HumanReviewed:
		return "human_reviewed"
	default:
		return "review"
	}
}

// Provider returns the provider that produced the authorizing review.
func (a Authority) Provider() string {
	if a.HasReceipt {
		return a.Receipt.Body.Reviewer.Provider
	}
	return a.ReviewEntry.Provider
}

// Verdict returns the authorizing review verdict.
func (a Authority) Verdict() string {
	if a.HasReceipt {
		return a.Receipt.Body.Verdict
	}
	return a.ReviewEntry.Status
}

// Summary returns the best available human-readable authority summary.
func (a Authority) Summary() string {
	if a.HasReceipt {
		return "finalization receipt " + a.Receipt.Body.Verdict
	}
	if a.HasDossier && a.Dossier.Summary != "" {
		return a.Dossier.Summary
	}
	return a.ReviewEntry.Reason
}

// RefusalSuffix summarizes terminal authority for commands that cannot operate
// on archived work.
func (a Authority) RefusalSuffix() string {
	if !a.Completed {
		return ""
	}
	suffix := fmt.Sprintf("; completion authority %s (%s)", a.Status(), a.Kind())
	if a.Verdict() != "" {
		suffix += ": review " + a.Verdict()
		if a.Provider() != "" {
			suffix += " by " + a.Provider()
		}
	}
	if !a.Valid && a.Reason != "" {
		suffix += "; integrity error: " + a.Reason
	}
	return suffix
}

// CurrentReviewGate validates the latest review gate that can currently
// authorize completion. Later build, criterion, phase, approval, fail, cancel,
// baseline, or review-attempt evidence invalidates older reviews.
func CurrentReviewGate(ledger session.Session) Authority {
	return reviewGateBefore(ledger, len(ledger.Entries), session.Entry{})
}

// TerminalAuthority validates the review gate that authorized the latest
// complete event in an archived task ledger.
func TerminalAuthority(ledger session.Session) Authority {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "complete" {
			continue
		}
		auth := reviewGateBefore(ledger, i, entry)
		auth.Completed = true
		auth.CompleteEntry = entry
		return auth
	}
	return Authority{
		Reason: "task has no completion event",
		Actual: "no complete session entry",
	}
}

func reviewGateBefore(ledger session.Session, end int, completeEntry session.Entry) Authority {
	for i := end - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case session.EntryReceipt:
			return validateReceiptEntry(ledger, entry, completeEntry)
		case "review":
			return validateReviewEntry(ledger, i, entry, completeEntry)
		case "review_attempt", "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return Authority{
				Completed:     completeEntry.Type == "complete",
				CompleteEntry: completeEntry,
				Reason:        "latest review gate has not passed",
				Actual:        "no current accepted review",
				Evidence:      []string{entryReference(entry)},
			}
		}
	}
	return Authority{
		Completed:     completeEntry.Type == "complete",
		CompleteEntry: completeEntry,
		Reason:        "latest review gate has not passed",
		Actual:        "no current accepted review",
		Evidence:      []string{"session review entries"},
	}
}

func validateReceiptEntry(ledger session.Session, entry session.Entry, completeEntry session.Entry) Authority {
	auth := Authority{
		Found:         true,
		Completed:     completeEntry.Type == "complete",
		ReceiptEntry:  entry,
		CompleteEntry: completeEntry,
		Evidence:      []string{entryReference(entry)},
	}
	if !ledger.LedgerValid {
		auth.Reason = "receipt ledger failed replay"
		auth.Actual = ledger.LedgerError
		return auth
	}
	envelope, err := receipt.DecodeEnvelope([]byte(entry.Output))
	if err != nil {
		auth.Reason = "latest receipt is invalid"
		auth.Actual = err.Error()
		return auth
	}
	auth.Receipt = envelope
	auth.HasReceipt = true
	if entry.Status != "pass" || envelope.Body.Verdict != "pass" {
		auth.Reason = "latest finalization receipt has not passed"
		auth.Actual = "receipt verdict " + envelope.Body.Verdict
		return auth
	}
	if !corereview.ValidCompletionProvider(envelope.Body.Reviewer.Provider) {
		auth.Reason = "passing receipt came from an unsupported provider"
		auth.Actual = fmt.Sprintf("provider %q", envelope.Body.Reviewer.Provider)
		return auth
	}
	if envelope.Body.MutationGuard.Status != "clean" {
		auth.Reason = "receipt mutation guard is not clean"
		auth.Actual = "mutation guard " + envelope.Body.MutationGuard.Status
		return auth
	}
	if len(envelope.Body.OpenBlockers) > 0 {
		auth.Reason = "latest receipt still has open completion blockers"
		auth.Actual = fmt.Sprintf("%d open blocker(s)", len(envelope.Body.OpenBlockers))
		return auth
	}
	for _, acceptance := range envelope.Body.Acceptance {
		if acceptance.Status != "pass" {
			auth.Reason = "latest receipt has failing acceptance"
			auth.Actual = fmt.Sprintf("%s: %s", acceptance.ID, acceptance.Status)
			return auth
		}
	}
	auth.Valid = true
	auth.Reason = "completion authorized by finalization receipt"
	auth.Actual = "receipt verdict pass"
	return auth
}

func validateReviewEntry(ledger session.Session, idx int, entry session.Entry, completeEntry session.Entry) Authority {
	auth := Authority{
		Found:         true,
		Completed:     completeEntry.Type == "complete",
		ReviewEntry:   entry,
		CompleteEntry: completeEntry,
		Evidence:      []string{entryReference(entry)},
	}
	if entry.Status != corereview.VerdictPass {
		auth.Reason = "latest review gate has not passed"
		auth.Actual = "latest review verdict " + entry.Status
		return auth
	}
	if entry.Provider != "human" {
		if stale := priorBlockingReviewWithoutRepairEvidence(ledger, idx); stale != "" {
			auth.Reason = "passing review is missing repair evidence after a blocking review"
			auth.Actual = "latest passing review follows " + stale + " without changed workspace or build evidence"
			auth.Evidence = append(auth.Evidence, stale)
			return auth
		}
	}
	if !corereview.ValidCompletionProvider(entry.Provider) {
		auth.Reason = "passing review came from an unsupported provider"
		auth.Actual = fmt.Sprintf("provider %q", entry.Provider)
		return auth
	}
	packet := strings.TrimSpace(entry.ReviewPacket)
	if entry.Provider != "human" {
		if packet == "" {
			auth.Reason = "latest review packet is missing"
			auth.Actual = "review entry has no review_packet"
			return auth
		}
		if strings.TrimSpace(entry.Output) != "" && strings.TrimSpace(entry.Output) != packet {
			auth.Reason = "latest review packet does not match session output"
			auth.Actual = "review_packet differs from output"
			return auth
		}
		if entry.CanonicalResponseSHA256 == "" {
			auth.Reason = "latest review packet hash is missing"
			auth.Actual = "review entry has no canonical_response_sha256"
			return auth
		}
		if got := corereview.ResponseSHA256(packet); got != entry.CanonicalResponseSHA256 {
			auth.Reason = "latest review packet hash mismatch"
			auth.Actual = fmt.Sprintf("canonical_response_sha256 %s, computed %s", entry.CanonicalResponseSHA256, got)
			return auth
		}
		if entry.ReviewedHead == "" || entry.ReviewedDirty == "" || entry.ReviewedDiff == "" {
			auth.Reason = "latest review workspace seal is incomplete"
			auth.Actual = "review entry must include reviewed_head, reviewed_dirty, and reviewed_diff"
			return auth
		}
		if !durableReviewedHead(entry.ReviewedHead) {
			auth.Reason = "latest review workspace head is not durable"
			auth.Actual = "reviewed_head " + entry.ReviewedHead
			return auth
		}
	}
	if packet == "" {
		packet = strings.TrimSpace(entry.Output)
	}
	if packet != "" {
		dossier, ok := corereview.DecodeDossier(packet)
		if !ok {
			auth.Reason = "latest review dossier is invalid"
			auth.Actual = "review packet could not be decoded as ReviewDossier"
			return auth
		}
		auth.Dossier = dossier
		auth.HasDossier = true
		if blockers := corereview.OpenBlockerCount(dossier.Findings); blockers > 0 {
			auth.Reason = "latest review dossier still has open completion blockers"
			auth.Actual = fmt.Sprintf("%d open blocker(s)", blockers)
			return auth
		}
	}
	if entry.Provider == "human" {
		auth.HumanReviewed = true
		if !hasAuditedHumanOverride(ledger, idx) {
			auth.Reason = "human-reviewed gate is missing audit evidence"
			auth.Actual = "latest human review has no adjacent review_override entry"
			return auth
		}
	}
	auth.Valid = true
	auth.Reason = "completion authorized by " + auth.Kind()
	auth.Actual = "review verdict pass"
	return auth
}

func durableReviewedHead(head string) bool {
	head = strings.TrimSpace(head)
	if head == "" || head == "unavailable" || strings.HasPrefix(head, "error:") {
		return false
	}
	return true
}

func priorBlockingReviewWithoutRepairEvidence(ledger session.Session, reviewIdx int) string {
	for i := reviewIdx - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "build", "criterion", "phase":
			return ""
		case "review":
			switch entry.Status {
			case corereview.VerdictPass:
				return ""
			case corereview.VerdictFail:
				if reviewAttemptChangedBetween(ledger, i, reviewIdx) {
					return ""
				}
				return entryReference(entry)
			}
		case "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return ""
		}
	}
	return ""
}

func reviewAttemptChangedBetween(ledger session.Session, failedReviewIdx int, passingReviewIdx int) bool {
	failedAttempt, okFailed := latestReviewAttemptBefore(ledger, failedReviewIdx)
	passingAttempt, okPassing := latestReviewAttemptBefore(ledger, passingReviewIdx)
	if !okFailed || !okPassing {
		return false
	}
	failedOutput := strings.TrimSpace(failedAttempt.Output)
	passingOutput := strings.TrimSpace(passingAttempt.Output)
	return failedOutput != "" && passingOutput != "" && failedOutput != passingOutput
}

func latestReviewAttemptBefore(ledger session.Session, reviewIdx int) (session.Entry, bool) {
	for i := reviewIdx - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review_attempt":
			return entry, true
		case "review", "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return session.Entry{}, false
		}
	}
	return session.Entry{}, false
}

func hasAuditedHumanOverride(ledger session.Session, reviewIdx int) bool {
	if reviewIdx == 0 {
		return false
	}
	previous := ledger.Entries[reviewIdx-1]
	return previous.Type == "review_override" && previous.Provider == "human" && previous.Status == "accepted"
}

func entryReference(entry session.Entry) string {
	if entry.ID != "" {
		return entry.ID
	}
	if entry.Type != "" {
		return entry.Type + " session entry"
	}
	return "session entry"
}
