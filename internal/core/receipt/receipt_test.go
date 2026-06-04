package receipt

import (
	"encoding/json"
	"os"
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

func TestReceiptConformanceCorpusPinsCanonicalBytesAndDigest(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("testdata/receipt_body.json")
	if err != nil {
		t.Fatal(err)
	}
	var body Body
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	canonical, err := CanonicalBody(body)
	if err != nil {
		t.Fatal(err)
	}
	wantCanonical, err := os.ReadFile("testdata/receipt_body.canonical.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(canonical) != strings.TrimSpace(string(wantCanonical)) {
		t.Fatalf("canonical receipt bytes drifted\nwant: %s\n got: %s", strings.TrimSpace(string(wantCanonical)), canonical)
	}

	// ReceiptDigest deliberately clears ledger_head before hashing: signatures
	// cover ledger_head, but the receipt digest cannot recursively include the
	// ledger head that is derived from that digest.
	digest, err := ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	wantDigest, err := os.ReadFile("testdata/receipt_body.digest")
	if err != nil {
		t.Fatal(err)
	}
	if digest != strings.TrimSpace(string(wantDigest)) {
		t.Fatalf("receipt digest drifted: want %s got %s", strings.TrimSpace(string(wantDigest)), digest)
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

func TestValidateReviewCoverageRejectsMismatchedProvenanceDigest(t *testing.T) {
	t.Parallel()

	body := validBody()
	// a.go is nominally covered, but the signed digest differs from the digest of
	// the bytes the reviewer actually saw. The receipt must be rejected.
	body.FileDigests = map[string]string{"a.go": "signed-digest"}
	body.ReviewedContextProvenance = []Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "reviewed-digest"}}
	body.IgnoredUnreviewed = []string{}
	if err := ValidateBody(body); err == nil || !strings.Contains(err.Error(), "do not match reviewed provenance digest") {
		t.Fatalf("a signed digest differing from the reviewed digest must be invalid, got %v", err)
	}
	// When the signed digest equals the reviewed digest, coverage is satisfied.
	body.ReviewedContextProvenance = []Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "signed-digest"}}
	if err := ValidateBody(body); err != nil {
		t.Fatalf("matching signed and reviewed digests must validate: %v", err)
	}
}

func validBody() Body {
	return Body{
		SchemaVersion:             SchemaVersion,
		TaskID:                    "task",
		SessionID:                 "session",
		Verdict:                   "pass",
		SnapshotMode:              SnapshotModeWorkingTree,
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"internal/core"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"b.go": "two", "a.go": "one"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "one", Bytes: 12}, {Kind: "evidence_file", Path: "b.go", SHA256: "two", Bytes: 12}},
		Reviewer:                  Reviewer{Provider: "codex", Model: "gpt"},
		HostUnderReview:           HostUnderReview{Agent: "codex"},
		Independence:              Independence{Level: IndependenceLevelIsolationOnly, Downgraded: IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        false,
		Acceptance:                []Acceptance{},
		OpenBlockers:              []Blocker{},
		MutationGuard:             MutationGuard{Status: "clean"},
		LedgerHead:                "ledger",
		MintedAt:                  "2026-06-03T00:00:00Z",
	}
}
