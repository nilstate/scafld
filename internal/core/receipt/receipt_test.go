package receipt

import (
	"strings"
	"testing"
)

func TestCanonicalEncodingSortsKeysDeterministically(t *testing.T) {
	t.Parallel()

	left := map[string]any{"z": "last", "a": map[string]any{"b": "two", "a": "one"}}
	right := map[string]any{"a": map[string]any{"a": "one", "b": "two"}, "z": "last"}
	leftJSON, err := CanonicalJSON(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, err := CanonicalJSON(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftJSON) != `{"a":{"a":"one","b":"two"},"z":"last"}` {
		t.Fatalf("canonical json = %s", leftJSON)
	}
	if string(leftJSON) != string(rightJSON) {
		t.Fatalf("canonical output differs:\n%s\n%s", leftJSON, rightJSON)
	}
}

func TestReceiptDigestExcludesDetachedSignature(t *testing.T) {
	t.Parallel()

	body := validBody()
	first := Envelope{Body: body, Signature: DetachedSignature{Alg: SignatureAlgorithm, KeyID: "one", Sig: "a"}}
	second := Envelope{Body: body, Signature: DetachedSignature{Alg: SignatureAlgorithm, KeyID: "two", Sig: "b"}}
	firstDigest, err := ReceiptDigest(first.Body)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := ReceiptDigest(second.Body)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == "" || firstDigest != secondDigest {
		t.Fatalf("digest should ignore signature: %q %q", firstDigest, secondDigest)
	}
}

func TestValidateRejectsMissingRequiredField(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.TaskID = ""
	err := ValidateBody(body)
	if err == nil || !strings.Contains(err.Error(), "task_id") {
		t.Fatalf("ValidateBody error = %v", err)
	}
}

func TestValidateReviewCoverageRejectsUnreviewedDigest(t *testing.T) {
	t.Parallel()

	body := validBody()
	body.FileDigests = map[string]string{"unreviewed.go": "sha"}
	body.ReviewedContextProvenance = []Provenance{}
	body.IgnoredUnreviewed = []string{}
	if err := ValidateBody(body); err == nil || !strings.Contains(err.Error(), "not covered") {
		t.Fatalf("a signed digest that is neither reviewed nor ignored must be invalid, got %v", err)
	}
	// Declaring it ignored_unreviewed makes the receipt honest and valid.
	body.IgnoredUnreviewed = []string{"unreviewed.go"}
	if err := ValidateBody(body); err != nil {
		t.Fatalf("declaring a digest ignored_unreviewed must satisfy coverage: %v", err)
	}
}

func validBody() Body {
	return Body{
		SchemaVersion:             SchemaVersion,
		TaskID:                    "task",
		SessionID:                 "session",
		Verdict:                   "pass",
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"internal/core"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"b.go": "two", "a.go": "one"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "one", Bytes: 12}, {Kind: "evidence_file", Path: "b.go", SHA256: "two", Bytes: 12}},
		Reviewer:                  Reviewer{Provider: "codex", Model: "gpt"},
		HostUnderReview:           HostUnderReview{Agent: "codex"},
		Independence:              Independence{Level: "isolation_only"},
		SpecFingerprint:           "spec",
		Acceptance:                []Acceptance{},
		OpenBlockers:              []Blocker{},
		MutationGuard:             MutationGuard{Status: "clean"},
		LedgerHead:                "ledger",
		MintedAt:                  "2026-06-03T00:00:00Z",
	}
}
