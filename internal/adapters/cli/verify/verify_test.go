package verify

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

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

func TestSelfCheckReportsWiringWithoutClaimingEnforcement(t *testing.T) {
	t.Parallel()

	opts, err := Parse([]string{"--self-check", "--root", "."})
	if err != nil || !opts.SelfCheck {
		t.Fatalf("Parse(--self-check) opts=%+v err=%v", opts, err)
	}

	// Default workspace: no workflow, policy local, no gap, and no enforcement claim.
	root := t.TempDir()
	report, err := SelfCheck(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if report.WorkflowInstalled || report.Policy != "local" || report.Gap != "" {
		t.Fatalf("default self-check report = %+v, want local/uninstalled/no-gap", report)
	}
	text := RenderSelfCheck(report)
	for _, want := range []string{"not installed", "scafld init --ci", "trusted keys:", "signing key:", "cannot read or set", "does not assert that any merge gate is active"} {
		if !strings.Contains(text, want) {
			t.Fatalf("self-check text missing %q:\n%s", want, text)
		}
	}

	// required policy without an installed workflow is surfaced as a gap, not hidden.
	gapRoot := t.TempDir()
	writeVerifyPolicyConfig(t, gapRoot, "required")
	gapReport, err := SelfCheck(context.Background(), gapRoot)
	if err != nil {
		t.Fatal(err)
	}
	if gapReport.Gap == "" || !strings.Contains(RenderSelfCheck(gapReport), "gap:") {
		t.Fatalf("required policy without workflow did not surface a gap: %+v", gapReport)
	}

	// An installed workflow is reported as installed.
	wfRoot := t.TempDir()
	writeVerifyWorkflow(t, wfRoot)
	wfReport, err := SelfCheck(context.Background(), wfRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !wfReport.WorkflowInstalled || !strings.Contains(RenderSelfCheck(wfReport), "installed (") {
		t.Fatalf("installed workflow not reported: %+v", wfReport)
	}

	// Handler path exits zero and prints the report.
	var stdout, stderr bytes.Buffer
	code := Handler()(context.Background(), []string{"--self-check", "--root", root}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("self-check exit = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "verify.policy: local") {
		t.Fatalf("handler stdout missing policy line:\n%s", stdout.String())
	}
}

func TestSelfCheckReportsTrustedKeyLifecycle(t *testing.T) {
	root := t.TempDir()
	activePub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatal(err)
	}
	revokedPub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("b", 64)))
	if err != nil {
		t.Fatal(err)
	}
	expiredPub, _, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("c", 64)))
	if err != nil {
		t.Fatal(err)
	}
	keys := trust.TrustedKeys{Version: trust.TrustedKeysVersion, Keys: []trust.TrustedKey{
		verifyTrustedKey(t, activePub),
		func() trust.TrustedKey {
			key := verifyTrustedKey(t, revokedPub)
			key.RevokedAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
			return key
		}(),
		func() trust.TrustedKey {
			key := verifyTrustedKey(t, expiredPub)
			key.ExpiresAt = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
			return key
		}(),
	}}
	writeTrustedKeys(t, root, keys)

	report, err := SelfCheck(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if report.TrustedKeysStatus != "valid" {
		t.Fatalf("trusted key status = %q", report.TrustedKeysStatus)
	}
	if report.KeyLifecycle.Active != 1 || report.KeyLifecycle.Revoked != 1 || report.KeyLifecycle.Expired != 1 {
		t.Fatalf("lifecycle = %+v", report.KeyLifecycle)
	}
	text := RenderSelfCheck(report)
	if !strings.Contains(text, "key lifecycle: active=1 revoked=1 expired=1") {
		t.Fatalf("rendered lifecycle missing:\n%s", text)
	}
}

func writeVerifyPolicyConfig(t *testing.T, root string, policy string) {
	t.Helper()
	dir := filepath.Join(root, ".scafld")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("verify:\n  policy: "+policy+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeVerifyWorkflow(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "scafld-verify.yml"), []byte("name: scafld-verify\n"), 0o644); err != nil {
		t.Fatal(err)
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

func TestRunMissingTrustedKeysFailsClosedInCI(t *testing.T) {
	t.Parallel()

	result, err := Run(context.Background(), Options{Root: t.TempDir(), ReceiptPath: "missing.json", Target: "main", CI: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || !strings.Contains(result.Reason, "trusted keys") {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunUsesReceiptPathFromEnvironment(t *testing.T) {
	// Hermetic: clear the ambient CI env (GitHub Actions sets CI=true) so this
	// exercises the non-CI receipt-path resolution, not the CI-policy branch.
	t.Setenv("CI", "")
	t.Setenv("SCAFLD_RECEIPT_PATH", "env-receipt.json")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	keys := trust.TrustedKeys{Version: trust.TrustedKeysVersion}
	keysJSON, err := trust.MarshalTrustedKeys(keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "trusted-keys.json"), keysJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = Run(context.Background(), Options{Root: root})
	if err == nil || !strings.Contains(err.Error(), "env-receipt.json") {
		t.Fatalf("verify must resolve SCAFLD_RECEIPT_PATH before config default, got %v", err)
	}
}

func TestSignatureVerifierRejectsRevokedAtAndExpiredKeys(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(strings.NewReader(strings.Repeat("z", 64)))
	if err != nil {
		t.Fatal(err)
	}
	baseKey := verifyTrustedKey(t, pub)
	for _, tt := range []struct {
		name string
		key  func(trust.TrustedKey) trust.TrustedKey
		want string
	}{
		{name: "revoked_at", want: "revoked", key: func(key trust.TrustedKey) trust.TrustedKey {
			key.RevokedAt = "2026-06-03T00:00:00Z"
			return key
		}},
		{name: "expires_at", want: "expired", key: func(key trust.TrustedKey) trust.TrustedKey {
			key.ExpiresAt = "2026-06-03T00:00:00Z"
			return key
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := minimalReceiptBody()
			body.MintedAt = "2026-06-03T00:00:00Z"
			envelope := signVerifyReceipt(t, body, baseKey.KeyID, priv)
			keys := trust.TrustedKeys{Version: trust.TrustedKeysVersion, Keys: []trust.TrustedKey{tt.key(baseKey)}}
			if err := (signatureVerifier{}).Verify(envelope, keys); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Verify error = %v, want %q", err, tt.want)
			}
		})
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
		SnapshotMode:              receipt.SnapshotModeWorkingTree,
		BaseCommit:                head,
		HeadCommit:                head,
		Scope:                     []string{"."},
		TreeSHA:                   "tampered-tree-sha",
		FileDigests:               map[string]string{"a.go": "sha"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "sha"}},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        false,
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
	exit := Handler()(context.Background(), []string{"--root", root, receiptPath, "--target", head, "--trusted-keys", filepath.Join(root, ".scafld", "trusted-keys.json")}, &stdout, &stderr)
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

func writeTrustedKeys(t *testing.T, root string, keys trust.TrustedKeys) {
	t.Helper()
	data, err := trust.MarshalTrustedKeys(keys)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "trusted-keys.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func verifyTrustedKey(t *testing.T, pub ed25519.PublicKey) trust.TrustedKey {
	t.Helper()
	keyID, err := trust.KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return trust.TrustedKey{
		KeyID:     keyID,
		Alg:       trust.AlgorithmEd25519,
		PublicKey: base64.StdEncoding.EncodeToString(pub),
	}
}

func signVerifyReceipt(t *testing.T, body receipt.Body, keyID string, privateKey ed25519.PrivateKey) receipt.Envelope {
	t.Helper()
	canonical, err := receipt.CanonicalBody(body)
	if err != nil {
		t.Fatal(err)
	}
	return receipt.Envelope{Body: body, Signature: receipt.DetachedSignature{
		Alg:   receipt.SignatureAlgorithm,
		KeyID: keyID,
		Sig:   base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, canonical)),
	}}
}

func minimalReceiptBody() receipt.Body {
	return receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    "task",
		SessionID:                 "session",
		Verdict:                   "pass",
		SnapshotMode:              receipt.SnapshotModeWorkingTree,
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"."},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"a.go": "sha"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "a.go", SHA256: "sha"}},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        false,
		Acceptance:                []receipt.Acceptance{},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		LedgerHead:                "ledger",
		MintedAt:                  "2026-06-03T00:00:00Z",
	}
}
