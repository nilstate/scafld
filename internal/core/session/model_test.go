package session

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
)

func TestReplayProjectionIdempotentAndAppendOrder(t *testing.T) {
	t.Parallel()

	ledger := New("task", "t0")
	ledger = ledger.WithEntry(Entry{ID: "one", Type: "criterion", RecordedAt: "t1", CriterionID: "ac1", Status: "fail"})
	ledger = ledger.WithEntry(Entry{ID: "two", Type: "criterion", RecordedAt: "t2", CriterionID: "ac1", Status: "pass"})
	replayed := Replay(ledger)
	if len(replayed.Entries) != 2 {
		t.Fatalf("entry count = %d", len(replayed.Entries))
	}
	if replayed.CriterionStates["ac1"].Status != "pass" {
		t.Fatalf("state = %+v", replayed.CriterionStates["ac1"])
	}
}

func TestReplayCriterionPhaseIDDoesNotOverwritePhaseBlock(t *testing.T) {
	t.Parallel()

	ledger := New("task", "t0")
	ledger = ledger.WithEntry(Entry{ID: "phase", Type: "phase", PhaseID: "phase1", Status: "completed", Reason: "phase done"})
	ledger = ledger.WithEntry(Entry{ID: "criterion", Type: "criterion", CriterionID: "ac1", PhaseID: "phase1", Status: "pass", Reason: "criterion pass"})
	replayed := Replay(ledger)
	if got := replayed.CriterionStates["ac1"].Status; got != "pass" {
		t.Fatalf("criterion state = %q, want pass", got)
	}
	if got := replayed.PhaseBlocks["phase1"].Status; got != "completed" {
		t.Fatalf("phase block was overwritten by criterion phase_id: %+v", replayed.PhaseBlocks["phase1"])
	}
}

func TestLedgerReplayComputesGenesisDigestNextHeadAndMismatch(t *testing.T) {
	t.Parallel()

	genesis := LedgerGenesisHead()
	if genesis == "" || genesis != New("task", "t0").LedgerHead {
		t.Fatalf("genesis = %q", genesis)
	}

	firstBody := validReceiptBody("task", genesis)
	firstDigest, err := receipt.ReceiptDigest(firstBody)
	if err != nil {
		t.Fatal(err)
	}
	firstHead := NextLedgerHead(genesis, firstDigest)
	firstBody.LedgerHead = firstHead
	firstEntry := receiptEntry(t, firstBody)

	secondBody := validReceiptBody("task", firstHead)
	secondDigest, err := receipt.ReceiptDigest(secondBody)
	if err != nil {
		t.Fatal(err)
	}
	secondHead := NextLedgerHead(firstHead, secondDigest)
	secondBody.LedgerHead = secondHead
	secondEntry := receiptEntry(t, secondBody)

	ledger := New("task", "t0").WithEntry(firstEntry).WithEntry(secondEntry)
	replayed := Replay(ledger)
	if !replayed.LedgerValid {
		t.Fatalf("ledger invalid: %s", replayed.LedgerError)
	}
	if replayed.LedgerHead != secondHead {
		t.Fatalf("ledger_head = %q, want %q", replayed.LedgerHead, secondHead)
	}

	ledger.Entries[1].LedgerHead = "wrong"
	replayed = Replay(ledger)
	if replayed.LedgerValid || replayed.LedgerError == "" {
		t.Fatalf("mismatch should invalidate ledger: %+v", replayed)
	}
}

func TestReplayWithOptionsChecksReceiptTrust(t *testing.T) {
	t.Parallel()

	body := validReceiptBody("task", LedgerGenesisHead())
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	body.LedgerHead = NextLedgerHead(LedgerGenesisHead(), digest)
	ledger := New("task", "t0").WithEntry(receiptEntry(t, body))

	replayed := ReplayWithOptions(ledger, ReplayOptions{ReceiptTrustChecker: rejectingTrustChecker{err: errors.New("trusted key expired")}})
	if replayed.LedgerValid || !strings.Contains(replayed.LedgerError, "trusted key expired") {
		t.Fatalf("trust rejection should invalidate ledger: %+v", replayed)
	}
	if replayed.Entries[0].TrustStatus != "rejected" || replayed.Entries[0].TrustReason == "" {
		t.Fatalf("entry trust status not stamped: %+v", replayed.Entries[0])
	}

	replayed = ReplayWithOptions(ledger, ReplayOptions{ReceiptTrustChecker: rejectingTrustChecker{}})
	if !replayed.LedgerValid {
		t.Fatalf("accepted trust should keep ledger valid: %s", replayed.LedgerError)
	}
	if replayed.Entries[0].TrustStatus != "accepted" || replayed.Entries[0].TrustReason != "" {
		t.Fatalf("accepted trust status not stamped: %+v", replayed.Entries[0])
	}
}

func TestReplayWithOptionsDoesNotMutateInputEntries(t *testing.T) {
	t.Parallel()

	body := validReceiptBody("task", LedgerGenesisHead())
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	body.LedgerHead = NextLedgerHead(LedgerGenesisHead(), digest)
	ledger := New("task", "t0").WithEntry(receiptEntry(t, body))

	_ = ReplayWithOptions(ledger, ReplayOptions{ReceiptTrustChecker: rejectingTrustChecker{}})
	if ledger.Entries[0].TrustStatus != "" || ledger.Entries[0].TrustReason != "" {
		t.Fatalf("ReplayWithOptions mutated input entry: %+v", ledger.Entries[0])
	}
}

func TestReplayWithOptionsCanSkipReceiptDecodeForMetadata(t *testing.T) {
	t.Parallel()

	ledger := New("task", "t0").WithEntry(Entry{
		Type:          EntryReceipt,
		RecordedAt:    "t1",
		ReceiptDigest: "digest",
		LedgerHead:    NextLedgerHead(LedgerGenesisHead(), "digest"),
		Output:        "{not valid receipt json",
	})
	replayed := ReplayWithOptions(ledger, ReplayOptions{SkipReceiptDecode: true})
	if !replayed.LedgerValid {
		t.Fatalf("metadata replay should trust stored digest chain, got %s", replayed.LedgerError)
	}
	full := Replay(ledger)
	if full.LedgerValid || !strings.Contains(full.LedgerError, "invalid receipt entry") {
		t.Fatalf("full replay should still reject invalid receipt output: %+v", full)
	}
}

func receiptEntry(t *testing.T, body receipt.Body) Entry {
	t.Helper()
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	envelope := receipt.Envelope{
		Body:      body,
		Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"},
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return Entry{Type: EntryReceipt, Output: string(data), ReceiptDigest: digest, LedgerHead: body.LedgerHead}
}

type rejectingTrustChecker struct{ err error }

func (r rejectingTrustChecker) CheckReceiptTrust(receipt.Envelope) error { return r.err }

func validReceiptBody(taskID string, priorHead string) receipt.Body {
	body := receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    taskID,
		SessionID:                 "session",
		Verdict:                   "pass",
		SnapshotMode:              receipt.SnapshotModeWorkingTree,
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"internal/core"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"a.go": "one"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "one"}},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        false,
		Acceptance:                []receipt.Acceptance{},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		LedgerHead:                priorHead,
		MintedAt:                  "2026-06-03T00:00:00Z",
	}
	return body
}
