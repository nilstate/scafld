package e2e

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/sign"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	appgate "github.com/nilstate/scafld/v2/internal/app/gate"
	appverify "github.com/nilstate/scafld/v2/internal/app/verify"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

// TestGateMintsReceiptVerifyAcceptsAndRejectsTamper exercises the full
// accountability round-trip with everything real except the reviewer port:
// snapshot -> acceptance -> sign -> receipt, then verify recomputes the tree,
// re-runs acceptance, and checks the signature. A post-mint tree change must
// flip verify to fail. This is the wired-path coverage the original stub lacked.
func TestGateMintsReceiptVerifyAcceptsAndRejectsTamper(t *testing.T) {
	root := initRepo(t)
	writeFile(t, root, "file.txt", "v2\n")

	keyPath, trusted := newSigningKey(t)
	runner := process.Runner{}
	input := appgate.Input{
		TaskID:          "demo",
		SessionID:       "demo",
		SpecFingerprint: "spec",
		HostUnderReview: receipt.HostUnderReview{Agent: "unknown"},
		Independence:    receipt.Independence{Level: "isolation_only", Distinct: false},
		Criteria:        []appacceptance.Criterion{{ID: "ac1", Command: "true", ExpectedKind: "exit_code_zero"}},
		WorkDir:         root,
		MintedAt:        time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC),
	}

	out, err := appgate.Run(context.Background(),
		gateSnap{g: git.Adapter{Root: root}},
		gateAccept{runner: runner},
		stubReviewer{g: git.Adapter{Root: root}},
		sign.Ed25519Signer{PrivateKeyPath: keyPath},
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass || out.Receipt == nil {
		t.Fatalf("gate did not mint a passing receipt: verdict=%q receipt=%v reason=%q", out.Verdict, out.Receipt, out.Reason)
	}
	if out.Receipt.Body.LedgerHead == "" || out.Receipt.Signature.Sig == "" {
		t.Fatalf("receipt missing ledger head or signature: %+v", out.Receipt)
	}

	ports := appverify.Ports{
		Snapshotter:       verifySnap{g: git.Adapter{Root: root}},
		AcceptanceRunner:  verifyAccept{runner: runner, root: root},
		AncestryChecker:   git.Adapter{Root: root},
		SignatureVerifier: verifySig{},
	}
	res, err := appverify.Run(context.Background(), *out.Receipt, trusted, appverify.Policy{}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("verify rejected an honest receipt: %s", res.Reason)
	}

	// Tamper the working tree after minting: verify must fail closed on the
	// recomputed tree fingerprint, since the receipt no longer matches reality.
	writeFile(t, root, "file.txt", "tampered\n")
	tampered, err := appverify.Run(context.Background(), *out.Receipt, trusted, appverify.Policy{}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if tampered.Passed || !strings.Contains(tampered.Reason, "tree mismatch") {
		t.Fatalf("verify accepted a tampered tree: passed=%v reason=%q", tampered.Passed, tampered.Reason)
	}
}

// --- gate ports ---

type stubReviewer struct{ g git.Adapter }

// Review stands in for a real reviewer binary but, like the real reviewer, returns
// provenance covering every reviewed file in the snapshot tree so the minted
// receipt is coverage-complete.
func (s stubReviewer) Review(ctx context.Context, in appgate.ReviewInput) (appgate.ReviewResult, error) {
	digests, err := s.g.TreeDigests(ctx, in.TreeSHA, in.Scope)
	if err != nil {
		return appgate.ReviewResult{}, err
	}
	prov := make([]receipt.Provenance, 0, len(digests))
	for _, d := range digests {
		prov = append(prov, receipt.Provenance{Kind: "evidence_file", Path: d.Path, SHA256: d.SHA256})
	}
	return appgate.ReviewResult{Dossier: review.Dossier{Verdict: review.VerdictPass}, Provenance: prov, Reviewer: receipt.Reviewer{Provider: "codex"}}, nil
}

type gateSnap struct{ g git.Adapter }

func (s gateSnap) Snapshot(ctx context.Context, in appgate.SnapshotInput) (appgate.Snapshot, error) {
	snap, err := s.g.Snapshot(ctx, git.SnapshotInput{Scope: in.Scope, BaseRef: in.BaseRef})
	if err != nil {
		return appgate.Snapshot{}, err
	}
	digests := map[string]string{}
	for _, d := range snap.FileDigests {
		digests[d.Path] = d.SHA256
	}
	ignored := []string{}
	for _, ig := range snap.IgnoredUnreviewed {
		ignored = append(ignored, ig.Path)
	}
	return appgate.Snapshot{TreeSHA: snap.TreeSHA, BaseCommit: snap.BaseCommit, HeadCommit: snap.HeadCommit, FileDigests: digests, IgnoredUnreviewed: ignored}, nil
}

type gateAccept struct{ runner appacceptance.Runner }

func (a gateAccept) Evaluate(ctx context.Context, in appacceptance.EvaluateInput) (appacceptance.EvaluateOutput, error) {
	return appacceptance.Evaluate(ctx, a.runner, in), nil
}

// --- verify ports ---

type verifySnap struct{ g git.Adapter }

func (s verifySnap) Snapshot(ctx context.Context, in appverify.SnapshotInput) (appverify.Snapshot, error) {
	snap, err := s.g.Snapshot(ctx, git.SnapshotInput{Scope: in.Scope})
	if err != nil {
		return appverify.Snapshot{}, err
	}
	digests := map[string]string{}
	for _, d := range snap.FileDigests {
		digests[d.Path] = d.SHA256
	}
	ignored := []string{}
	for _, ig := range snap.IgnoredUnreviewed {
		ignored = append(ignored, ig.Path)
	}
	return appverify.Snapshot{TreeSHA: snap.TreeSHA, FileDigests: digests, Ignored: ignored}, nil
}

type verifyAccept struct {
	runner appacceptance.Runner
	root   string
}

func (a verifyAccept) RunAcceptance(ctx context.Context, criteria []receipt.Acceptance) ([]appverify.AcceptanceResult, error) {
	out := make([]appverify.AcceptanceResult, 0, len(criteria))
	for _, c := range criteria {
		ev := appacceptance.Evaluate(ctx, a.runner, appacceptance.EvaluateInput{
			Criteria: []appacceptance.Criterion{{ID: c.ID, Command: c.Command, ExpectedKind: c.ExpectedKind}},
			WorkDir:  a.root,
		})
		if len(ev.Results) == 0 {
			continue
		}
		out = append(out, appverify.AcceptanceResult{ID: ev.Results[0].ID, Status: ev.Results[0].Status, ExitCode: ev.Results[0].ExitCode})
	}
	return out, nil
}

type verifySig struct{}

func (verifySig) Verify(envelope receipt.Envelope, trusted trust.TrustedKeys) error {
	key, err := trusted.ActiveKey(envelope.Signature.KeyID)
	if err != nil {
		return err
	}
	pub, err := key.PublicKeyBytes()
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(envelope.Signature.Sig)
	if err != nil {
		return err
	}
	canonical, err := receipt.CanonicalBody(envelope.Body)
	if err != nil {
		return err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), canonical, sig) {
		return errors.New("invalid signature")
	}
	return nil
}

// --- helpers ---

func newSigningKey(t *testing.T) (string, trust.TrustedKeys) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(t.TempDir(), "receipt.key")
	if err := os.WriteFile(keyPath, priv, 0o600); err != nil {
		t.Fatal(err)
	}
	keyID, err := trust.KeyIDFromRawEd25519PublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	trusted := trust.TrustedKeys{Version: trust.TrustedKeysVersion, Keys: []trust.TrustedKey{
		{KeyID: keyID, Alg: trust.AlgorithmEd25519, PublicKey: base64.StdEncoding.EncodeToString(pub)},
	}}
	return keyPath, trusted
}

func initRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	runGit(t, root, "config", "user.name", "scafld")
	runGit(t, root, "config", "user.email", "scafld@example.invalid")
	writeFile(t, root, "file.txt", "base\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "base")
	return root
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
