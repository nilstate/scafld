package completion

import (
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/testkit/sessiontest"
)

func validReceiptBody(taskID string, priorHead string) receipt.Body {
	body := receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    taskID,
		SessionID:                 taskID,
		Verdict:                   "pass",
		SnapshotMode:              receipt.SnapshotModeWorkingTree,
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"file.go"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"file.go": "sha"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "file.go", SHA256: "sha"}},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        true,
		Acceptance:                []receipt.Acceptance{{ID: "ac1", Status: "pass"}},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		MintedAt:                  "2026-06-04T00:00:00Z",
	}
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		panic(err)
	}
	body.LedgerHead = session.NextLedgerHead(priorHead, digest)
	return body
}

func TestTerminalAuthorityUsesLatestPassingReviewBeforeCompletion(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "build-1", Type: "build", Status: "review", Reason: "review repair evidence refreshed"})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-2", "codex"))
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})

	authority := TerminalAuthority(ledger)
	if !authority.Completed || !authority.Valid || authority.Kind() != "review" || authority.Provider() != "codex" || authority.Verdict() != corereview.VerdictPass {
		t.Fatalf("authority = %+v", authority)
	}
	if authority.ReviewEntry.ID != "review-2" || authority.CompleteEntry.ID != "complete-1" {
		t.Fatalf("authority events = review %q complete %q", authority.ReviewEntry.ID, authority.CompleteEntry.ID)
	}
}

func TestTerminalAuthorityAcceptsPassingFinalizationReceipt(t *testing.T) {
	t.Parallel()

	body := validReceiptBody("task", session.LedgerGenesisHead())
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	body.LedgerHead = session.NextLedgerHead(session.LedgerGenesisHead(), digest)
	envelope := receipt.Envelope{
		Body:      body,
		Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"},
	}
	output, err := receipt.CanonicalJSON(envelope)
	if err != nil {
		t.Fatal(err)
	}
	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "receipt-1", Type: session.EntryReceipt, Status: "pass", ReceiptDigest: digest, LedgerHead: body.LedgerHead, Output: string(output)})
	ledger = ledger.WithEntry(session.Entry{ID: "complete-1", Type: "complete", Status: "completed"})

	authority := TerminalAuthority(ledger)
	if !authority.Completed || !authority.Valid || authority.Kind() != "receipt" || authority.Provider() != "codex" || authority.Verdict() != "pass" {
		t.Fatalf("authority = %+v", authority)
	}
	if authority.ReceiptEntry.ID != "receipt-1" || authority.CompleteEntry.ID != "complete-1" {
		t.Fatalf("authority events = receipt %q complete %q", authority.ReceiptEntry.ID, authority.CompleteEntry.ID)
	}
}

func TestCurrentReviewGateRequiresRepairEvidenceAfterBlockingReview(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-2", "codex"))

	authority := CurrentReviewGate(ledger)
	if authority.Valid || authority.Reason != "passing review is missing repair evidence after a blocking review" {
		t.Fatalf("authority = %+v", authority)
	}
	if authority.Actual != "latest passing review follows review-1 without changed workspace or build evidence" {
		t.Fatalf("actual = %q", authority.Actual)
	}
}

func TestCurrentReviewGateAcceptsReviewRepairAfterChangedAttemptEvidence(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "attempt-1", Type: "review_attempt", Status: "running", Output: "task_changes_since_baseline:\n- changed file.go (old)"})
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "attempt-2", Type: "review_attempt", Status: "running", Output: "task_changes_since_baseline:\n- changed file.go (new)"})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-2", "codex"))

	authority := CurrentReviewGate(ledger)
	if !authority.Valid || authority.ReviewEntry.ID != "review-2" {
		t.Fatalf("authority = %+v", authority)
	}
}

func TestCurrentReviewGateRejectsReviewRerollWithSameAttemptEvidence(t *testing.T) {
	t.Parallel()

	output := "task_changes_since_baseline:\n- changed file.go (same)"
	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "attempt-1", Type: "review_attempt", Status: "running", Output: output})
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "attempt-2", Type: "review_attempt", Status: "running", Output: output})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-2", "codex"))

	authority := CurrentReviewGate(ledger)
	if authority.Valid || authority.Reason != "passing review is missing repair evidence after a blocking review" {
		t.Fatalf("authority = %+v", authority)
	}
}

func TestCurrentReviewGateAcceptsReviewRepairAfterBuildEvidence(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{ID: "review-1", Type: "review", Status: corereview.VerdictFail, Provider: "codex"})
	ledger = ledger.WithEntry(session.Entry{ID: "build-1", Type: "build", Status: "review", Reason: "review repair evidence refreshed"})
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("review-2", "codex"))

	authority := CurrentReviewGate(ledger)
	if !authority.Valid || authority.ReviewEntry.ID != "review-2" {
		t.Fatalf("authority = %+v", authority)
	}
}

func TestCurrentReviewGateRejectsSentinelReviewedHead(t *testing.T) {
	t.Parallel()

	for _, head := range []string{"unavailable", "error: git failed"} {
		ledger := session.New("task", "now")
		entry := sessiontest.PassingReviewEntry("review-1", "codex")
		entry.ReviewedHead = head
		ledger = ledger.WithEntry(entry)

		authority := CurrentReviewGate(ledger)
		if authority.Valid || authority.Reason != "latest review workspace head is not durable" {
			t.Fatalf("head %q authority = %+v", head, authority)
		}
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
	ledger = ledger.WithEntry(sessiontest.PassingReviewEntry("", "codex"))
	ledger = ledger.WithEntry(session.Entry{Type: "build", Status: "active", Reason: "repair started"})

	authority := CurrentReviewGate(ledger)
	if authority.Valid || authority.Found || authority.Actual != "no current accepted review" {
		t.Fatalf("authority = %+v", authority)
	}
}
