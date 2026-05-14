package completion

import (
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
)

func cleanDossier(provider string) corereview.Dossier {
	return corereview.Dossier{
		Verdict:  corereview.VerdictPass,
		Mode:     corereview.ModeVerify,
		Provider: provider,
		Summary:  "review passed",
		AttackLog: []corereview.AttackLogEntry{{
			Target: "diff",
			Attack: "scan",
			Result: corereview.AttackResultClean,
		}},
	}
}

func TestTerminalAuthorityUsesLatestPassingReviewBeforeCompletion(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "review-2", Type: "review", Status: corereview.VerdictPass, Provider: "codex", Output: corereview.EncodeDossier(cleanDossier("codex"))})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})

	authority := TerminalAuthority(ledger)
	if !authority.Completed || !authority.Valid || authority.Kind() != "review" || authority.Provider() != "codex" || authority.Verdict() != corereview.VerdictPass {
		t.Fatalf("authority = %+v", authority)
	}
	if authority.ReviewEntry.ID != "review-2" || authority.CompleteEntry.ID != "complete-1" {
		t.Fatalf("authority events = review %q complete %q", authority.ReviewEntry.ID, authority.CompleteEntry.ID)
	}
}

func TestTerminalAuthorityAcceptsAuditedHumanReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "override-1", Type: "review_override", Status: "accepted", Provider: "human", Reason: "manual audit"})
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictPass, Provider: "human", Reason: "human-reviewed override: manual audit"})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})

	authority := TerminalAuthority(ledger)
	if !authority.Valid || !authority.HumanReviewed || authority.Kind() != "human_reviewed" || authority.Summary() != "human-reviewed override: manual audit" {
		t.Fatalf("authority = %+v", authority)
	}
}

func TestTerminalAuthorityFlagsCompletedTaskWithoutPassingReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})

	authority := TerminalAuthority(ledger)
	if !authority.Completed || authority.Valid || authority.Kind() != "invalid" || authority.Reason != "latest review gate has not passed" {
		t.Fatalf("authority = %+v", authority)
	}
	if authority.Actual != "latest review verdict fail" {
		t.Fatalf("actual = %q", authority.Actual)
	}
}

func TestCurrentReviewGateInvalidatesStaleReviewAfterLaterBuildEvidence(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{Type: "review", Status: corereview.VerdictPass, Provider: "codex", Output: corereview.EncodeDossier(cleanDossier("codex"))})
	ledger = ledger.WithEntry(session.Entry{Type: "build", Status: "active", Reason: "repair started"})

	authority := CurrentReviewGate(ledger)
	if authority.Valid || authority.Found || authority.Actual != "no current accepted review" {
		t.Fatalf("authority = %+v", authority)
	}
}
