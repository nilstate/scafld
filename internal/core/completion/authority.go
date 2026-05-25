// Package completion evaluates the review authority that allows a task to close.
package completion

import (
	"fmt"

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
	CompleteEntry session.Entry
	Dossier       corereview.Dossier
	HasDossier    bool
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
	case a.HumanReviewed:
		return "human_reviewed"
	default:
		return "review"
	}
}

// Provider returns the provider that produced the authorizing review.
func (a Authority) Provider() string {
	return a.ReviewEntry.Provider
}

// Verdict returns the authorizing review verdict.
func (a Authority) Verdict() string {
	return a.ReviewEntry.Status
}

// Summary returns the best available human-readable authority summary.
func (a Authority) Summary() string {
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
		if stale := priorBlockingReviewWithoutBuild(ledger, idx); stale != "" {
			auth.Reason = "passing review is missing refreshed build evidence after a blocking review"
			auth.Actual = "latest passing review follows " + stale + " without intervening build evidence"
			auth.Evidence = append(auth.Evidence, stale)
			return auth
		}
	}
	if !corereview.ValidCompletionProvider(entry.Provider) {
		auth.Reason = "passing review came from an unsupported provider"
		auth.Actual = fmt.Sprintf("provider %q", entry.Provider)
		return auth
	}
	if entry.Output != "" {
		dossier, ok := corereview.DecodeDossier(entry.Output)
		if !ok {
			auth.Reason = "latest review dossier is invalid"
			auth.Actual = "review entry output could not be decoded as ReviewDossier"
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

func priorBlockingReviewWithoutBuild(ledger session.Session, reviewIdx int) string {
	for i := reviewIdx - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "build", "criterion", "phase":
			return ""
		case "review":
			if entry.Status == corereview.VerdictFail {
				return entryReference(entry)
			}
		case "approval", session.EntryWorkspaceBaseline, "fail", "cancel", "complete":
			return ""
		}
	}
	return ""
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
