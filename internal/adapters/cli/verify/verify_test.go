package verify

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"os"
	"os/exec"
	"path/filepath"

	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

func TestParseTarget(t *testing.T) {
	t.Parallel()

	opts, err := Parse([]string{"receipt.json", "--target", "main", "--ci"})
	if err != nil {
		t.Fatal(err)
	}
	if opts.ReceiptPath != "receipt.json" || opts.Target != "main" || !opts.CI {
		t.Fatalf("opts = %+v", opts)
	}
}

func TestRunMissingTargetFailsClosedInCI(t *testing.T) {
	t.Parallel()

	result, err := Run(context.Background(), Options{Root: t.TempDir(), ReceiptPath: "missing.json", CI: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "missing target") {
		t.Fatalf("result = %+v", result)
	}
}

func TestTamperedTreeVerifyExitsNonzero(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	gitCmd(t, root, "init")
	gitCmd(t, root, "config", "user.name", "scafld")
	gitCmd(t, root, "config", "user.email", "scafld@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, root, "add", "a.go")
	gitCmd(t, root, "commit", "-m", "base")
	head := gitCmd(t, root, "rev-parse", "HEAD")

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := trust.KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	keys := trust.TrustedKeys{Version: trust.TrustedKeysVersion, Keys: []trust.TrustedKey{{
		KeyID:     keyID,
		Alg:       trust.AlgorithmEd25519,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}}}
	keysJSON, err := trust.MarshalTrustedKeys(keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "trusted-keys.json"), keysJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	body := receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    "task",
		SessionID:                 "session",
		Verdict:                   "pass",
		BaseCommit:                head,
		HeadCommit:                head,
		Scope:                     []string{"."},
		TreeSHA:                   "tampered-tree-sha",
		FileDigests:               map[string]string{"a.go": "sha"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "sha"}},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: "isolation_only"},
		SpecFingerprint:           "spec",
		Acceptance:                []receipt.Acceptance{},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		LedgerHead:                "ledger",
		MintedAt:                  "2026-06-03T00:00:00Z",
	}
	canonical, err := receipt.CanonicalBody(body)
	if err != nil {
		t.Fatal(err)
	}
	envelope := receipt.Envelope{Body: body, Signature: receipt.DetachedSignature{
		Alg:   receipt.SignatureAlgorithm,
		KeyID: keyID,
		Sig:   base64.StdEncoding.EncodeToString(ed25519.Sign(priv, canonical)),
	}}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(receiptPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := Handler()(context.Background(), []string{"--root", root, receiptPath, "--target", head}, &stdout, &stderr)
	if exit == 0 || !strings.Contains(stderr.String(), "tree mismatch") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func gitCmd(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
