package finalize

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	appfinalize "github.com/nilstate/scafld/v2/internal/app/finalize"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestGateScopeUsesSpecScopeAndTouchpoints(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Scope:       []string{"internal/app/finalize"},
		Touchpoints: []string{"internal/core/receipt/receipt.go"},
		Context:     spec.Context{Packages: []string{"internal/core/trust"}},
	}
	scope := gateScope(model, []string{"internal/adapters/sign"})
	want := []string{
		"internal/adapters/sign",
		"internal/app/finalize",
		"internal/core/receipt/receipt.go",
		"internal/core/trust",
	}
	if !reflect.DeepEqual(scope, want) {
		t.Fatalf("gateScope = %v, want %v", scope, want)
	}
	// A spec that declares only Scope must still produce a non-empty gate scope,
	// so the gate never falls back to whole-repo gating.
	if got := gateScope(spec.Model{Scope: []string{"internal/app/finalize"}}, nil); len(got) == 0 {
		t.Fatal("spec Scope must yield a non-empty gate scope")
	}
}

func TestGateScopeFiltersProseAndSplitsTouchpointPaths(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Scope: []string{
			"In scope: end-to-end tests, initwire defaults, and verify workflow defaults.",
			"Out of scope: real hosted provider execution in CI.",
			"internal/app/finalize",
		},
		Touchpoints: []string{
			"`docs/review.md`, `docs/sourcey.config.ts`: docs nav and review guidance",
			"test/e2e/",
		},
	}
	scope := gateScope(model, nil)
	want := []string{"docs/review.md", "docs/sourcey.config.ts", "internal/app/finalize", "test/e2e"}
	if !reflect.DeepEqual(scope, want) {
		t.Fatalf("gateScope = %v, want %v", scope, want)
	}
}

type fakeBaseDiff struct {
	paths  []string
	called int
}

func (f *fakeBaseDiff) BaseDiffPaths(context.Context, string) ([]string, error) {
	f.called++
	return append([]string(nil), f.paths...), nil
}

func TestNoSpecScopeHintSynthesizesModel(t *testing.T) {
	t.Parallel()

	req := Request{TaskID: "missing-no-spec", ScopeHint: []string{"internal/adapters/cli/finalize"}}
	model, hasSpec, err := loadGateModel(context.Background(), markdown.Store{Root: t.TempDir()}, req)
	if err != nil {
		t.Fatal(err)
	}
	if hasSpec || model.TaskID != req.TaskID || !strings.Contains(model.Summary, "No hand-authored spec") {
		t.Fatalf("model=%+v hasSpec=%v, want synthesized no-spec model", model, hasSpec)
	}
	scope, err := deriveGateScope(context.Background(), &fakeBaseDiff{}, model, req, hasSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope, []string{"internal/adapters/cli/finalize"}) {
		t.Fatalf("scope = %v", scope)
	}
}

func TestNoSpecEmptyScopeRefuses(t *testing.T) {
	t.Parallel()

	req := Request{TaskID: "missing-no-spec"}
	model, hasSpec, err := loadGateModel(context.Background(), markdown.Store{Root: t.TempDir()}, req)
	if err != nil {
		t.Fatal(err)
	}
	_, err = deriveGateScope(context.Background(), &fakeBaseDiff{}, model, req, hasSpec)
	if err == nil || !strings.Contains(err.Error(), "finalize scope is empty") {
		t.Fatalf("empty no-spec scope error = %v", err)
	}
}

func TestNoSpecBaseDiffScopeTopLevelExtensionless(t *testing.T) {
	t.Parallel()

	req := Request{TaskID: "missing-no-spec", BaseRef: "origin/main"}
	model, hasSpec, err := loadGateModel(context.Background(), markdown.Store{Root: t.TempDir()}, req)
	if err != nil {
		t.Fatal(err)
	}
	// Top-level extensionless changed files (Makefile, Dockerfile) must survive
	// no-spec base-diff scope synthesis instead of being prose-filtered away.
	diff := &fakeBaseDiff{paths: []string{"Makefile", "Dockerfile", "src/a.go"}}
	scope, err := deriveGateScope(context.Background(), diff, model, req, hasSpec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope, []string{"Dockerfile", "Makefile", "src/a.go"}) {
		t.Fatalf("scope = %v, want top-level extensionless files retained", scope)
	}
}

func TestNoSpecBaseDiffScope(t *testing.T) {
	t.Parallel()

	req := Request{TaskID: "missing-no-spec", BaseRef: "origin/main"}
	model, hasSpec, err := loadGateModel(context.Background(), markdown.Store{Root: t.TempDir()}, req)
	if err != nil {
		t.Fatal(err)
	}
	diff := &fakeBaseDiff{paths: []string{"z.go", "a.go", "a.go"}}
	scope, err := deriveGateScope(context.Background(), diff, model, req, hasSpec)
	if err != nil {
		t.Fatal(err)
	}
	if diff.called != 1 || !reflect.DeepEqual(scope, []string{"a.go", "z.go"}) {
		t.Fatalf("diff called=%d scope=%v, want base-diff scope", diff.called, scope)
	}
}

func TestNoSpecMissingSessionStartsNewLedger(t *testing.T) {
	t.Parallel()

	ledger, err := loadGateLedger(context.Background(), jsonstore.SessionStore{Root: t.TempDir()}, "missing-no-spec", time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if ledger.TaskID != "missing-no-spec" || ledger.LedgerHead != session.LedgerGenesisHead() {
		t.Fatalf("ledger = %+v, want new genesis ledger", ledger)
	}
}

func TestGateRunErrorsOnMissingTaskID(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := Run(context.Background(), []string{"--json", "--stdin"}, strings.NewReader("{}"), &out)
	if err == nil {
		t.Fatal("gate must return a tool error when task_id is missing, not a success payload")
	}
}

func TestReadRequestParsesBaseRef(t *testing.T) {
	t.Parallel()

	req, err := readRequest(options{Stdin: true}, strings.NewReader(`{"task_id":"demo","base_ref":"origin/main"}`))
	if err != nil {
		t.Fatal(err)
	}
	if req.BaseRef != "origin/main" {
		t.Fatalf("base_ref = %q, want origin/main", req.BaseRef)
	}
}

func TestSelectReviewerRequiresConfiguredAbsoluteBinary(t *testing.T) {
	t.Parallel()

	// No configured reviewer binary: the gate must fail closed even though
	// codex/claude/gemini may be on PATH (PATH is host-controlled).
	if _, _, err := selectReviewerWithEnv(configadapter.Config{}, ".", "claude", process.Runner{}, nil); err == nil {
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

func TestSelectReviewerAcceptsFinalizeEnvBinary(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, runtime, err := selectReviewerWithEnv(configadapter.Config{}, ".", "claude", process.Runner{}, []string{
		"SCAFLD_FINALIZE_CODEX_BINARY=" + bin,
		"SCAFLD_FINALIZE_CODEX_MODEL=gpt-finalize",
	})
	if err != nil {
		t.Fatalf("env absolute reviewer binary should be accepted: %v", err)
	}
	if runtime.Binary != bin || runtime.Provider != "codex" || runtime.Model != "gpt-finalize" {
		t.Fatalf("runtime = %+v, want codex %q model gpt-finalize", runtime, bin)
	}
}

func TestSelectReviewerEnvDoesNotOverrideConfiguredBinary(t *testing.T) {
	configured := filepath.Join(t.TempDir(), "configured-codex")
	if err := os.WriteFile(configured, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	envBin := filepath.Join(t.TempDir(), "env-codex")
	if err := os.WriteFile(envBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := configadapter.Config{Review: configadapter.ReviewConfig{External: configadapter.ExternalReviewConfig{
		Codex: configadapter.ProviderConfig{Binary: configured, Model: "configured-model"},
	}}}

	_, runtime, err := selectReviewerWithEnv(cfg, ".", "claude", process.Runner{}, []string{
		"SCAFLD_FINALIZE_CODEX_BINARY=" + envBin,
		"SCAFLD_FINALIZE_CODEX_MODEL=env-model",
	})
	if err != nil {
		t.Fatalf("configured reviewer binary should be accepted: %v", err)
	}
	if runtime.Binary != configured || runtime.Model != "configured-model" {
		t.Fatalf("runtime = %+v, want configured binary/model", runtime)
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
	snap, err := g.Snapshot(context.Background(), git.SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	_, provenance, _, err := buildEvidence(context.Background(), g, snap.TreeSHA, []string{"."}, []string{"removed.go"})
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
	snap, err := g.Snapshot(context.Background(), git.SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	files, provenance, ignored, err := buildEvidence(context.Background(), g, snap.TreeSHA, []string{"."}, nil)
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
		SnapshotMode:              receipt.SnapshotModeWorkingTree,
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"internal/adapters/cli/finalize"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{},
		Reviewer:                  receipt.Reviewer{Provider: "codex"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence:              receipt.Independence{Level: receipt.IndependenceLevelIsolationOnly, Downgraded: receipt.IndependenceDowngradeSameVendor},
		SpecFingerprint:           "spec",
		AcceptanceDeclared:        false,
		Acceptance:                []receipt.Acceptance{},
		OpenBlockers:              []receipt.Blocker{},
		MutationGuard:             receipt.MutationGuard{Status: "clean"},
		MintedAt:                  "2026-06-03T00:00:00Z",
		// A ledger_head that does not chain from the session's current head.
		LedgerHead: "this-head-does-not-chain",
	}
	env := receipt.Envelope{Body: body, Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"}}
	out := appfinalize.Output{Verdict: "pass", Receipt: &env}
	if _, err := finalize(context.Background(), tmp, "demo", jsonstore.SessionStore{Root: tmp}, spec.Model{TaskID: "demo"}, false, out); err == nil {
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

	out := appfinalize.Output{
		Verdict: "fail",
		Reason:  "acceptance failed before review",
		Independence: receipt.Independence{
			Level:      receipt.IndependenceLevelIsolationOnly,
			Downgraded: receipt.IndependenceDowngradeSameVendor,
		},
		Acceptance: appacceptance.EvaluateOutput{
			Passed: false,
			Results: []appacceptance.CriterionResult{
				{ID: "ac1", Command: "go test ./...", Status: "fail", ExitCode: 1, Reason: "exit code was 1"},
			},
		},
	}
	root := t.TempDir()
	resp, err := finalize(context.Background(), root, "demo", jsonstore.SessionStore{Root: root}, spec.Model{TaskID: "demo"}, false, out)
	if err != nil {
		t.Fatal(err)
	}
	results, ok := resp["acceptance"].([]map[string]any)
	if !ok || len(results) != 1 || results[0]["id"] != "ac1" || results[0]["status"] != "fail" {
		t.Fatalf("failed acceptance criterion details must be in the gate response: %+v", resp["acceptance"])
	}
	independence, ok := resp["independence"].(receipt.Independence)
	if !ok || independence.Downgraded != receipt.IndependenceDowngradeSameVendor {
		t.Fatalf("failed gate response independence = %#v", resp["independence"])
	}
}

func TestFinalizeReturnsGateResponseIndependence(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	env := validReceiptEnvelope(t, "demo")
	out := appfinalize.Output{Verdict: "pass", Receipt: &env}
	resp, err := finalize(context.Background(), tmp, "demo", jsonstore.SessionStore{Root: tmp}, spec.Model{TaskID: "demo"}, false, out)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := resp["independence"].(receipt.Independence)
	if !ok {
		t.Fatalf("independence response payload = %#v", resp["independence"])
	}
	if got.Level != receipt.IndependenceLevelCrossVendor || !got.Distinct || !strings.Contains(got.Reason, "multi-model") {
		t.Fatalf("independence payload = %+v", got)
	}
	receiptPath, ok := resp["receipt_path"].(string)
	if !ok || filepath.Base(receiptPath) != "latest.json" {
		t.Fatalf("receipt_path = %#v, want latest.json", resp["receipt_path"])
	}
	taskReceiptPath, ok := resp["task_receipt_path"].(string)
	if !ok || filepath.Base(taskReceiptPath) != "demo.json" {
		t.Fatalf("task_receipt_path = %#v, want demo.json", resp["task_receipt_path"])
	}
	for _, path := range []string{receiptPath, taskReceiptPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("receipt file %s missing: %v", path, err)
		}
	}
}

func validReceiptEnvelope(t *testing.T, taskID string) receipt.Envelope {
	t.Helper()
	body := receipt.Body{
		SchemaVersion:             receipt.SchemaVersion,
		TaskID:                    taskID,
		SessionID:                 taskID,
		Verdict:                   "pass",
		SnapshotMode:              receipt.SnapshotModeBaseDelta,
		BaseRef:                   "origin/main",
		BaseCommit:                "base",
		HeadCommit:                "head",
		Scope:                     []string{"internal/adapters/cli/finalize"},
		TreeSHA:                   "tree",
		FileDigests:               map[string]string{"internal/adapters/cli/finalize/run.go": "sha"},
		IgnoredUnreviewed:         []string{},
		ReviewedContextProvenance: []receipt.Provenance{{Kind: "evidence_file", Path: "internal/adapters/cli/finalize/run.go", SHA256: "sha"}},
		Reviewer:                  receipt.Reviewer{Provider: "claude"},
		HostUnderReview:           receipt.HostUnderReview{Agent: "codex"},
		Independence: receipt.Independence{
			Level:    receipt.IndependenceLevelCrossVendor,
			Distinct: true,
			Reason:   "cross_vendor: multi-model review reduces correlated blind spots but remains single-party local tooling",
		},
		SpecFingerprint:    "spec",
		AcceptanceDeclared: true,
		Acceptance:         []receipt.Acceptance{{ID: "ac1", Status: "pass", ExitCode: 0}},
		OpenBlockers:       []receipt.Blocker{},
		MutationGuard:      receipt.MutationGuard{Status: "clean"},
		MintedAt:           "2026-06-03T00:00:00Z",
	}
	digest, err := receipt.ReceiptDigest(body)
	if err != nil {
		t.Fatal(err)
	}
	body.LedgerHead = session.NextLedgerHead(session.LedgerGenesisHead(), digest)
	return receipt.Envelope{Body: body, Signature: receipt.DetachedSignature{Alg: receipt.SignatureAlgorithm, KeyID: "key", Sig: "sig"}}
}
