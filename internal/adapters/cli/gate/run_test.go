package gate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	appgate "github.com/nilstate/scafld/v2/internal/app/gate"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestGateScopeUsesSpecScopeAndTouchpoints(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Scope:       []string{"internal/app/gate"},
		Touchpoints: []string{"internal/core/receipt/receipt.go"},
		Context:     spec.Context{Packages: []string{"internal/core/trust"}},
	}
	scope := gateScope(model, []string{"internal/adapters/sign"})
	want := []string{
		"internal/adapters/sign",
		"internal/app/gate",
		"internal/core/receipt/receipt.go",
		"internal/core/trust",
	}
	if !reflect.DeepEqual(scope, want) {
		t.Fatalf("gateScope = %v, want %v", scope, want)
	}
	// A spec that declares only Scope must still produce a non-empty gate scope,
	// so the gate never falls back to whole-repo gating.
	if got := gateScope(spec.Model{Scope: []string{"internal/app/gate"}}, nil); len(got) == 0 {
		t.Fatal("spec Scope must yield a non-empty gate scope")
	}
}

func TestGateRunErrorsOnMissingTaskID(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := Run(context.Background(), []string{"--json", "--stdin"}, strings.NewReader("{}"), &out)
	if err == nil {
		t.Fatal("scafld_gate must return a tool error when task_id is missing, not a success payload")
	}
}

func TestSelectReviewerRequiresConfiguredAbsoluteBinary(t *testing.T) {
	t.Parallel()

	// No configured reviewer binary: the gate must fail closed even though
	// codex/claude/gemini may be on PATH (PATH is host-controlled).
	if _, _, err := selectReviewer(configadapter.Config{}, ".", "claude", process.Runner{}); err == nil {
		t.Fatal("gate must fail closed without a configured absolute reviewer binary")
	}

	// A configured absolute reviewer binary is accepted and used verbatim.
	bin := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := configadapter.Config{Review: configadapter.ReviewConfig{External: configadapter.ExternalReviewConfig{
		Codex: configadapter.ProviderConfig{Binary: bin},
	}}}
	_, runtime, err := selectReviewer(cfg, ".", "claude", process.Runner{})
	if err != nil {
		t.Fatalf("configured absolute reviewer binary should be accepted: %v", err)
	}
	if runtime.Binary != bin || runtime.Provider != "codex" {
		t.Fatalf("runtime = %q/%q, want codex %q", runtime.Provider, runtime.Binary, bin)
	}
}

func TestSpecFingerprintCoversTaskContract(t *testing.T) {
	t.Parallel()

	base := spec.Model{TaskID: "t", Summary: "original", Scope: []string{"x"}}
	changed := base
	changed.Summary = "materially different summary"
	if specFingerprint(base, base.Scope) == specFingerprint(changed, changed.Scope) {
		t.Fatal("spec_fingerprint must change when the approved task contract (summary) changes")
	}
}

func TestGateEvidenceIncludesDeletedPaths(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runGit("config", "user.name", "scafld")
	runGit("config", "user.email", "scafld@example.invalid")
	if err := os.WriteFile(filepath.Join(root, "keep.txt"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "base")

	g := git.Adapter{Root: root}
	snap, err := g.Snapshot(context.Background(), git.SnapshotInput{})
	if err != nil {
		t.Fatal(err)
	}
	_, provenance, _, err := buildEvidence(context.Background(), g, snap.TreeSHA, nil, []string{"removed.go"})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range provenance {
		if p.Kind == "deleted" && p.Path == "removed.go" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a deleted scoped path must appear as a tombstone in receipt provenance: %+v", provenance)
	}
}

func TestBuildEvidenceDoesNotSilentlyDropScopedGovernedFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	runGit("config", "user.name", "scafld")
	runGit("config", "user.email", "scafld@example.invalid")
	for name, content := range map[string]string{"CLAUDE.md": "instructions\n", "main.go": "package main\n"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	runGit("add", "-A")
	runGit("commit", "-m", "base")

	g := git.Adapter{Root: root}
	snap, err := g.Snapshot(context.Background(), git.SnapshotInput{})
	if err != nil {
		t.Fatal(err)
	}
	files, provenance, ignored, err := buildEvidence(context.Background(), g, snap.TreeSHA, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The governed file is signed in file_digests but must be declared ignored,
	// never shown to the reviewer as evidence, so no pass receipt can imply review.
	if !containsString(ignored, "CLAUDE.md") {
		t.Fatalf("governed file must be recorded as ignored_unreviewed: %v", ignored)
	}
	for _, f := range files {
		if f.Path == "CLAUDE.md" {
			t.Fatal("governed file must not be shown to the reviewer as evidence")
		}
	}
	for _, p := range provenance {
		if p.Path == "CLAUDE.md" {
			t.Fatal("governed file must not appear in reviewed provenance")
		}
	}
	// A normal scoped file is still reviewed evidence.
	reviewed := false
	for _, p := range provenance {
		if p.Kind == "evidence_file" && p.Path == "main.go" {
			reviewed = true
		}
	}
	if !reviewed {
		t.Fatalf("a normal file must be reviewed evidence: %+v", provenance)
	}
}

func TestFinalizeRejectsLedgerHeadMismatch(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	body := receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    "demo",
		SessionID:                 "demo",
		Verdict:                   "pass",
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: "isolation_only"},
		Acceptance:                []receipt.Acceptance{},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		MintedAt:                  "2026-06-03T00:00:00Z",
		// A ledger_head that does not chain from the session's current head.
		LedgerHead: "this-head-does-not-chain",
	}
	env := receipt.Envelope{Body: body, Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"}}
	out := appgate.Output{Verdict: "pass", Receipt: &env}
	if _, err := finalize(context.Background(), tmp, "demo", jsonstore.SessionStore{Root: tmp}, out); err == nil {
		t.Fatal("finalize must fail closed when the receipt ledger_head does not chain from the current head")
	}
}

func containsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func TestGateFailedAcceptanceCarriesCriterionDetails(t *testing.T) {
	t.Parallel()

	out := appgate.Output{
		Verdict: "fail",
		Reason:  "acceptance failed before review",
		Acceptance: appacceptance.EvaluateOutput{
			Passed: false,
			Results: []appacceptance.CriterionResult{
				{ID: "ac1", Command: "go test ./...", Status: "fail", ExitCode: 1, Reason: "exit code was 1"},
			},
		},
	}
	resp, err := finalize(context.Background(), t.TempDir(), "demo", jsonstore.SessionStore{Root: t.TempDir()}, out)
	if err != nil {
		t.Fatal(err)
	}
	results, ok := resp["acceptance"].([]map[string]any)
	if !ok || len(results) != 1 || results[0]["id"] != "ac1" || results[0]["status"] != "fail" {
		t.Fatalf("failed acceptance criterion details must be in the gate response: %+v", resp["acceptance"])
	}
}
