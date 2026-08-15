package finalize

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/sign"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	appfinalize "github.com/nilstate/scafld/v2/internal/app/finalize"
	appverify "github.com/nilstate/scafld/v2/internal/app/verify"
	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/reviewscope"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/core/trust"
)

func TestGateCriteriaCarriesMatchingManualEvidence(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "task", Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{
		ID: "images", Type: "manual", ExpectedKind: acceptance.ExpectedManual,
	}}}}
	ledger := session.New("task", "now").WithEntry(session.Entry{
		ID: "evidence-1", Type: session.EntryManualEvidence, CriterionID: "images", Status: "pass",
		ExpectedKind: string(acceptance.ExpectedManual), CriterionType: "manual", EvidenceDigest: strings.Repeat("c", 64),
		EvidenceActor: "operator", Reason: "verified", RecordedAt: "2026-08-14T00:00:00Z",
	})
	criteria := gateCriteria(model, ledger)
	if len(criteria) != 1 || criteria[0].ManualEvidence == nil {
		t.Fatalf("criteria=%+v, want matching manual evidence", criteria)
	}
	out := appacceptance.Evaluate(context.Background(), nil, appacceptance.EvaluateInput{Criteria: criteria})
	if !out.Passed || out.Results[0].EvidenceDigest != strings.Repeat("c", 64) {
		t.Fatalf("acceptance=%+v, want signed manual evidence result", out)
	}
}

func TestFinalizeGateErrorPropagatesAcceptanceBlocker(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		TaskID: "task", Status: spec.StatusBlocked,
		CurrentState: spec.CurrentState{CurrentPhase: "final", Reason: "final acceptance failed", AllowedFollowUp: "scafld handoff task"},
		Acceptance:   spec.Acceptance{Criteria: []spec.Criterion{{ID: "images", Type: "manual", Title: "Image identities", ExpectedKind: acceptance.ExpectedManual, Status: "pending", Evidence: "manual criterion requires human evidence"}}},
	}
	err := finalizeGateError(model, session.New("task", "now"), reviewgate.State{})
	var gateErr gate.Error
	if !errors.As(err, &gateErr) {
		t.Fatalf("error=%v, want structured gate error", err)
	}
	if gateErr.Failure.Reason != "final acceptance failed" || gateErr.Failure.Actual != "task status blocked" || len(gateErr.Failure.Blockers) != 1 || !strings.Contains(gateErr.Failure.Blockers[0], "images") {
		t.Fatalf("failure=%+v, want acceptance blocker details", gateErr.Failure)
	}
}

func TestGateScopeUsesSpecScopeAndTouchpoints(t *testing.T) {
	t.Parallel()

	model := spec.Model{
		Scope:       []string{"internal/app/finalize"},
		Touchpoints: []string{"internal/core/receipt/receipt.go"},
		Context:     spec.Context{Packages: []string{"internal/core/trust"}},
	}
	scope := reviewscope.Merge(reviewscope.Derive(model, nil, nil), reviewscope.Literal([]string{"internal/adapters/sign"}))
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
	if got := reviewscope.Derive(spec.Model{Scope: []string{"internal/app/finalize"}}, nil, nil); len(got) == 0 {
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
	scope := reviewscope.Derive(model, nil, nil)
	want := []string{"docs/review.md", "docs/sourcey.config.ts", "internal/app/finalize", "test/e2e"}
	if !reflect.DeepEqual(scope, want) {
		t.Fatalf("gateScope = %v, want %v", scope, want)
	}
}

func TestGateScopeUsesExplicitHintBeforeSpecAndRecordedReview(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "scoped-task", Scope: []string{"../app", "api/"}}
	ledger := session.New("scoped-task", "now").WithEntry(session.Entry{
		Type:          "review",
		Status:        review.VerdictPass,
		ReviewedSpec:  spec.ContractDigest(model),
		ReviewedScope: []string{"api"},
	})
	if got, err := deriveGateScope(context.Background(), &fakeBaseDiff{}, model, Request{TaskID: model.TaskID, ScopeHint: []string{"api/explicit.go"}}, true, ledger); err != nil || !reflect.DeepEqual(got, []string{"api/explicit.go"}) {
		t.Fatalf("explicit scope = %v err=%v", got, err)
	}
	if got, err := deriveGateScope(context.Background(), &fakeBaseDiff{}, model, Request{TaskID: model.TaskID}, true, ledger); err != nil || !reflect.DeepEqual(got, []string{"api"}) {
		t.Fatalf("recorded review scope = %v err=%v", got, err)
	}
}

func TestGateScopeDoesNotPassSiblingEvidenceRootToGit(t *testing.T) {
	t.Parallel()

	model := spec.Model{TaskID: "sibling-task", Scope: []string{"../app", "api/handler.go"}}
	scope, err := deriveGateScope(context.Background(), &fakeBaseDiff{}, model, Request{TaskID: model.TaskID}, true, session.New(model.TaskID, "now"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope, []string{"api/handler.go"}) {
		t.Fatalf("scope = %v, want only repository-relative API scope", scope)
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
	scope, err := deriveGateScope(context.Background(), &fakeBaseDiff{}, model, req, hasSpec, session.New(req.TaskID, "now"))
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
	_, err = deriveGateScope(context.Background(), &fakeBaseDiff{}, model, req, hasSpec, session.New(req.TaskID, "now"))
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
	scope, err := deriveGateScope(context.Background(), diff, model, req, hasSpec, session.New(req.TaskID, "now"))
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
	scope, err := deriveGateScope(context.Background(), diff, model, req, hasSpec, session.New(req.TaskID, "now"))
	if err != nil {
		t.Fatal(err)
	}
	if diff.called != 1 || !reflect.DeepEqual(scope, []string{"a.go", "z.go"}) {
		t.Fatalf("diff called=%d scope=%v, want base-diff scope", diff.called, scope)
	}
}

func TestSpecWithoutScopeUsesBaseDiffScope(t *testing.T) {
	t.Parallel()

	req := Request{TaskID: "scoped-by-diff", BaseRef: "origin/main"}
	model := spec.Model{TaskID: "scoped-by-diff", Title: "Scoped By Diff"}
	diff := &fakeBaseDiff{paths: []string{"Makefile", "internal/app/review/review.go"}}
	scope, err := deriveGateScope(context.Background(), diff, model, req, true, session.New(req.TaskID, "now"))
	if err != nil {
		t.Fatal(err)
	}
	if diff.called != 1 || !reflect.DeepEqual(scope, []string{"Makefile", "internal/app/review/review.go"}) {
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

func TestParseOptionsAcceptsPublicTaskID(t *testing.T) {
	t.Parallel()

	opts, err := parseOptions([]string{
		"runx-release-readiness-v1",
		"--json",
		"--root", "/repo",
		"--base-ref=origin/main",
		"--scope-hint", "internal/adapters/cli/finalize/run.go",
		"--scope-hint=internal/adapters/cli/finalize/run_test.go",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opts.TaskID != "runx-release-readiness-v1" || !opts.JSON || opts.Root != "/repo" || opts.BaseRef != "origin/main" {
		t.Fatalf("opts = %+v", opts)
	}
	wantScope := []string{"internal/adapters/cli/finalize/run.go", "internal/adapters/cli/finalize/run_test.go"}
	if !reflect.DeepEqual(opts.ScopeHint, wantScope) {
		t.Fatalf("scope hint = %v, want %v", opts.ScopeHint, wantScope)
	}
}

func TestParseOptionsRejectsMultipleTaskIDs(t *testing.T) {
	t.Parallel()

	_, err := parseOptions([]string{"one", "two", "--json"})
	if err == nil || !strings.Contains(err.Error(), "at most one task_id") {
		t.Fatalf("multiple task_id error = %v", err)
	}
}

func TestReadRequestUsesPublicTaskIDWithoutStdin(t *testing.T) {
	t.Parallel()

	req, err := readRequest(options{TaskID: "demo", Root: "/repo", BaseRef: "HEAD", ScopeHint: []string{"a.go"}}, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if req.TaskID != "demo" || req.Root != "/repo" || req.BaseRef != "HEAD" || !reflect.DeepEqual(req.ScopeHint, []string{"a.go"}) {
		t.Fatalf("request = %+v", req)
	}
}

func TestReadRequestRejectsConflictingTaskIDs(t *testing.T) {
	t.Parallel()

	_, err := readRequest(options{Stdin: true, TaskID: "cli-task"}, strings.NewReader(`{"task_id":"stdin-task"}`))
	if err == nil || !strings.Contains(err.Error(), "conflicts with stdin task_id") {
		t.Fatalf("conflicting task_id error = %v", err)
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

type fakeHeadResolver struct {
	head  string
	ok    bool
	err   error
	calls int
}

func (f *fakeHeadResolver) ResolveHead(context.Context) (string, bool, error) {
	f.calls++
	return f.head, f.ok, f.err
}

func TestBaseDeltaDefaultResolvesHEAD(t *testing.T) {
	t.Parallel()

	resolver := &fakeHeadResolver{head: "abc123", ok: true}
	got, err := defaultFinalizeBaseRef(context.Background(), resolver, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc123" {
		t.Fatalf("default base_ref = %q, want HEAD commit", got)
	}

	explicit, err := defaultFinalizeBaseRef(context.Background(), resolver, " origin/main ")
	if err != nil {
		t.Fatal(err)
	}
	if explicit != "origin/main" {
		t.Fatalf("explicit base_ref = %q, want trimmed caller value", explicit)
	}
	if resolver.calls != 1 {
		t.Fatalf("ResolveHead calls = %d, want only omitted base_ref to resolve HEAD", resolver.calls)
	}
}

func TestNoHeadWorkingTreeFallback(t *testing.T) {
	t.Parallel()

	got, err := defaultFinalizeBaseRef(context.Background(), &fakeHeadResolver{ok: false}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("no-HEAD default base_ref = %q, want empty working_tree fallback", got)
	}
}

func TestCommittedBaseDeltaSealVerifiesAfterCommit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := initFinalizeRepo(t)
	parent := strings.TrimSpace(finalizeGitOutput(t, root, "rev-parse", "HEAD"))
	writeFinalizeFile(t, root, "file.txt", "after\n")

	baseRef, err := defaultFinalizeBaseRef(ctx, git.Adapter{Root: root}, "")
	if err != nil {
		t.Fatal(err)
	}
	if baseRef != parent {
		t.Fatalf("default base_ref = %q, want parent HEAD %q", baseRef, parent)
	}

	out, trusted := mintTestReceipt(t, root, baseRef)
	if out.Receipt == nil {
		t.Fatal("finalize did not mint a receipt")
	}
	body := out.Receipt.Body
	if body.SnapshotMode != receipt.SnapshotModeBaseDelta || body.BaseRef != parent || body.BaseCommit != parent {
		t.Fatalf("receipt snapshot = mode %q base_ref %q base_commit %q, want base_delta against parent %q", body.SnapshotMode, body.BaseRef, body.BaseCommit, parent)
	}

	receiptBytes, err := json.MarshalIndent(out.Receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFinalizeFile(t, root, ".scafld/receipts/base-delta-seal.json", string(receiptBytes)+"\n")
	finalizeRunGit(t, root, "add", "-A")
	finalizeRunGit(t, root, "commit", "-m", "seal")

	ports := appverify.Ports{
		Snapshotter:       finalizeVerifySnapshotter{git: git.Adapter{Root: root}},
		AcceptanceRunner:  finalizeVerifyAcceptance{runner: process.Runner{}, root: root},
		AncestryChecker:   git.Adapter{Root: root},
		SignatureVerifier: finalizeVerifySignature{},
	}
	res, err := appverify.Run(ctx, *out.Receipt, trusted, appverify.Policy{TargetCommit: parent}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("committed base_delta receipt did not verify against parent: %s", res.Reason)
	}
}

func TestFinalizeIgnoresOutOfScopeMutationDuringAcceptance(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	root := initFinalizeRepo(t)
	writeFinalizeFile(t, root, "ambient.txt", "ambient before\n")
	finalizeRunGit(t, root, "add", "-A")
	finalizeRunGit(t, root, "commit", "-m", "ambient base")
	base := strings.TrimSpace(finalizeGitOutput(t, root, "rev-parse", "HEAD"))
	writeFinalizeFile(t, root, "file.txt", "task after\n")

	keyPath, trusted := newFinalizeSigningKey(t)
	out, err := appfinalize.Run(ctx,
		gateSnapshotter{git: git.Adapter{Root: root}},
		finalizeAcceptanceFunc(func(context.Context, appacceptance.EvaluateInput) (appacceptance.EvaluateOutput, error) {
			writeFinalizeFile(t, root, "ambient.txt", "ambient changed by another agent\n")
			return appacceptance.EvaluateOutput{
				Passed: true,
				Results: []appacceptance.CriterionResult{{
					ID:           "ac1",
					Command:      "true",
					ExpectedKind: "exit_code_zero",
					Status:       "pass",
					Reason:       "exit code was 0",
				}},
			}, nil
		}),
		sign.Ed25519Signer{PrivateKeyPath: keyPath},
		appfinalize.Input{
			TaskID:          "ambient-drift",
			SessionID:       "ambient-drift",
			Scope:           []string{"file.txt"},
			BaseRef:         base,
			SpecFingerprint: "spec",
			Review:          passingReviewEvidence(t, root, []string{"file.txt"}, base),
			HostUnderReview: receipt.HostUnderReview{Agent: "unknown"},
			Criteria:        []appacceptance.Criterion{{ID: "ac1", Command: "true", ExpectedKind: "exit_code_zero"}},
			WorkDir:         root,
			MintedAt:        time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass || out.Receipt == nil {
		t.Fatalf("finalize verdict=%q reason=%q receipt=%v", out.Verdict, out.Reason, out.Receipt != nil)
	}
	if _, ok := out.Receipt.Body.FileDigests["file.txt"]; !ok || len(out.Receipt.Body.FileDigests) != 1 {
		t.Fatalf("receipt file digests = %+v, want only scoped task file", out.Receipt.Body.FileDigests)
	}

	finalizeRunGit(t, root, "add", "-A")
	finalizeRunGit(t, root, "commit", "-m", "task plus ambient")
	head := strings.TrimSpace(finalizeGitOutput(t, root, "rev-parse", "HEAD"))
	ports := appverify.Ports{
		Snapshotter:       finalizeVerifySnapshotter{git: git.Adapter{Root: root}},
		AcceptanceRunner:  finalizeVerifyAcceptance{runner: process.Runner{}, root: root},
		AncestryChecker:   git.Adapter{Root: root},
		SignatureVerifier: finalizeVerifySignature{},
	}
	res, err := appverify.Run(ctx, *out.Receipt, trusted, appverify.Policy{TargetCommit: head}, ports)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Passed {
		t.Fatalf("scoped receipt did not verify after ambient commit: %s", res.Reason)
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

func buildEvidence(ctx context.Context, g git.Adapter, treeSHA string, scope []string, deleted []string, facts []appfinalize.FileFact) ([]reviewevidence.EvidenceFile, []receipt.Provenance, []string, error) {
	if len(facts) == 0 {
		digests, err := g.TreeDigests(ctx, treeSHA, scope)
		if err != nil {
			return nil, nil, nil, err
		}
		facts = make([]appfinalize.FileFact, 0, len(digests))
		for _, d := range digests {
			facts = append(facts, appfinalize.FileFact{Path: d.Path, Status: d.Status, SHA256: d.SHA256})
		}
	}
	var ignored []string
	reviewable := make([]appfinalize.FileFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Status == "gitlink" || blocklistedEvidence(fact.Path) {
			ignored = append(ignored, fact.Path)
			continue
		}
		reviewable = append(reviewable, fact)
	}
	paths := make([]string, 0, len(reviewable))
	for _, fact := range reviewable {
		paths = append(paths, fact.Path)
	}
	blobs, err := g.TreeBlobs(ctx, treeSHA, paths)
	if err != nil {
		return nil, nil, nil, err
	}
	files := make([]reviewevidence.EvidenceFile, 0, len(reviewable))
	provenance := make([]receipt.Provenance, 0, len(reviewable)+len(deleted))
	for _, fact := range reviewable {
		data, ok := blobs[fact.Path]
		if !ok {
			return nil, nil, nil, fmt.Errorf("missing evidence bytes for %s", fact.Path)
		}
		files = append(files, reviewevidence.EvidenceFile{Path: fact.Path, Status: fact.Status, SHA256: fact.SHA256, Bytes: data})
		provenance = append(provenance, receipt.Provenance{Kind: "evidence_file", Path: fact.Path, SHA256: fact.SHA256, Bytes: len(data)})
	}
	for _, path := range deleted {
		if blocklistedEvidence(path) {
			ignored = append(ignored, path)
			continue
		}
		provenance = append(provenance, receipt.Provenance{Kind: "deleted", Path: path})
	}
	if len(files) == 0 && len(provenance) == 0 {
		detail := "scope did not resolve to any file bytes or deletion tombstones"
		if len(ignored) > 0 {
			detail = "scope only contains signed but withheld paths: " + strings.Join(ignored, ", ")
		}
		return nil, nil, ignored, fmt.Errorf("finalize evidence is not reviewable: %s; run finalize in the owning repository or pass a scope that includes reviewable files", detail)
	}
	return files, provenance, ignored, nil
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
	_, provenance, _, err := buildEvidence(context.Background(), g, snap.TreeSHA, []string{"."}, []string{"removed.go"}, nil)
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
	files, provenance, ignored, err := buildEvidence(context.Background(), g, snap.TreeSHA, []string{"."}, nil, nil)
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

func TestBuildEvidenceRejectsEmptyReviewableEvidence(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "-A")
	runGit("commit", "-m", "base")

	g := git.Adapter{Root: root}
	snap, err := g.Snapshot(context.Background(), git.SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	_, _, ignored, err := buildEvidence(context.Background(), g, snap.TreeSHA, []string{"."}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "finalize evidence is not reviewable") {
		t.Fatalf("error = %v, want not-reviewable evidence failure", err)
	}
	if !containsString(ignored, "AGENTS.md") {
		t.Fatalf("ignored = %v, want AGENTS.md recorded", ignored)
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

func TestFinalizePassWithAcceptanceResultsAppendsReceipt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := jsonstore.SessionStore{Root: root}
	env := validReceiptEnvelope(t, "demo")
	out := appfinalize.Output{
		Verdict: "pass",
		Receipt: &env,
		Acceptance: appacceptance.EvaluateOutput{
			Passed: true,
			Results: []appacceptance.CriterionResult{
				{ID: "ac1", Command: "go test ./...", ExpectedKind: "exit_code_zero", Status: "pass", ExitCode: 0, Reason: "exit code was 0"},
			},
		},
	}
	if _, err := finalize(context.Background(), root, "demo", store, spec.Model{TaskID: "demo"}, false, out); err != nil {
		t.Fatal(err)
	}
	ledger, err := store.Load(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.LedgerValid {
		t.Fatalf("ledger invalid after acceptance+receipt append: %s", ledger.LedgerError)
	}
	if len(ledger.Entries) != 3 {
		t.Fatalf("entries = %+v, want criterion, receipt, complete", ledger.Entries)
	}
	if ledger.Entries[0].Type != "criterion" || ledger.Entries[1].Type != session.EntryReceipt || ledger.Entries[2].Type != "complete" {
		t.Fatalf("entry order = %+v, want criterion, receipt, complete", ledger.Entries)
	}
	if ledger.Entries[0].ExpectedKind != "exit_code_zero" || ledger.Entries[0].CriterionType != "command" {
		t.Fatalf("criterion entry = %+v, want expected_kind/type preserved", ledger.Entries[0])
	}
	if ledger.Entries[1].LedgerHead != env.Body.LedgerHead || ledger.LedgerHead != env.Body.LedgerHead {
		t.Fatalf("ledger head = %q entry head=%q want %q", ledger.LedgerHead, ledger.Entries[1].LedgerHead, env.Body.LedgerHead)
	}
	if got := ledger.CriterionStates["ac1"].Status; got != "pass" {
		t.Fatalf("criterion state = %q, want pass", got)
	}
}

func TestFinalizeArchivesSpecAndIsIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ctx := context.Background()
	specs := markdown.Store{Root: root}
	model := spec.Model{TaskID: "archive-task", Title: "Archive Task", Summary: "A completed task", Status: spec.StatusReview}
	specPath, err := specs.CreateDraft(ctx, model)
	if err != nil {
		t.Fatal(err)
	}
	store := jsonstore.SessionStore{Root: root}
	env := validReceiptEnvelope(t, "archive-task")
	out := appfinalize.Output{Verdict: review.VerdictPass, Receipt: &env}
	now := env.Body.MintedAt
	if _, err := finalizeWithArchive(ctx, root, model.TaskID, store, specs, model, true, specPath, out, now); err != nil {
		t.Fatal(err)
	}
	if _, err := finalizeWithArchive(ctx, root, model.TaskID, store, specs, model, true, "", out, now); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(root, ".scafld", "specs", "archive", "2026-06", "archive-task.md")
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatalf("completed spec was not archived: %v", err)
	}
	if _, err := os.Stat(specPath); !os.IsNotExist(err) {
		t.Fatalf("draft spec still exists after archive: %v", err)
	}
	archived, _, err := specs.Load(ctx, model.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != spec.StatusCompleted {
		t.Fatalf("archived status = %q, want completed", archived.Status)
	}
	ledger, err := store.Load(ctx, model.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Entries) != 2 || ledger.Entries[0].Type != session.EntryReceipt || ledger.Entries[1].Type != "complete" {
		t.Fatalf("idempotent finalization appended duplicate evidence: %+v", ledger.Entries)
	}
}

func TestFinalizeAcceptanceEvidencePreservesContractFieldsForReplay(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := jsonstore.SessionStore{Root: root}
	env := validReceiptEnvelope(t, "demo")
	model := spec.Model{
		TaskID: "demo",
		Status: spec.StatusReview,
		Phases: []spec.Phase{{
			ID:     "phase1",
			Name:   "Phase",
			Status: "completed",
			Acceptance: []spec.Criterion{{
				ID:           "ac1",
				Type:         "command",
				PhaseID:      "phase1",
				Command:      "go test ./...",
				ExpectedKind: "exit_code_zero",
				Status:       "pending",
			}},
		}},
	}
	out := appfinalize.Output{
		Verdict: "pass",
		Receipt: &env,
		Acceptance: appacceptance.EvaluateOutput{
			Passed: true,
			Results: []appacceptance.CriterionResult{{
				ID:           "ac1",
				Type:         "command",
				Command:      "go test ./...",
				ExpectedKind: "exit_code_zero",
				Status:       "pass",
				ExitCode:     0,
				Reason:       "exit code was 0",
			}},
		},
	}
	if _, err := finalize(context.Background(), root, "demo", store, model, false, out); err != nil {
		t.Fatal(err)
	}
	ledger, err := store.Load(context.Background(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	entry := ledger.Entries[0]
	if entry.ExpectedKind != "exit_code_zero" || entry.CriterionType != "command" || entry.PhaseID != "phase1" {
		t.Fatalf("criterion entry = %+v, want contract-bound evidence", entry)
	}
	projected := reconcile.FromSession(model, ledger)
	got := projected.Phases[0].Acceptance[0]
	if got.Status != "pass" || got.SourceEvent == "" {
		t.Fatalf("finalized criterion did not replay as current evidence: %+v", got)
	}
	if projected.Phases[0].Status != "completed" {
		t.Fatalf("finalized criterion evidence overwrote phase status: %+v", projected.Phases[0])
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

func passingReviewEvidence(t *testing.T, root string, scope []string, baseRef string) appfinalize.ReviewEvidence {
	t.Helper()
	snap, err := (gateSnapshotter{git: git.Adapter{Root: root}}).Snapshot(context.Background(), appfinalize.SnapshotInput{Scope: scope, BaseRef: baseRef})
	if err != nil {
		t.Fatal(err)
	}
	provenance, ignored := snapshotReviewCoverage(snap)
	return appfinalize.ReviewEvidence{
		Dossier:    review.Dossier{Verdict: review.VerdictPass},
		Provenance: provenance,
		Ignored:    ignored,
		Reviewer:   receipt.Reviewer{Provider: "codex", Model: "test"},
	}
}

type finalizeVerifySnapshotter struct{ git git.Adapter }

func (s finalizeVerifySnapshotter) Snapshot(ctx context.Context, in appverify.SnapshotInput) (appverify.Snapshot, error) {
	snap, err := s.git.Snapshot(ctx, git.SnapshotInput{Scope: in.Scope, BaseRef: in.BaseRef})
	if err != nil {
		return appverify.Snapshot{}, err
	}
	digests := make(map[string]string, len(snap.FileDigests))
	for _, d := range snap.FileDigests {
		digests[d.Path] = d.SHA256
	}
	ignored := make([]string, 0, len(snap.IgnoredUnreviewed))
	for _, item := range snap.IgnoredUnreviewed {
		ignored = append(ignored, item.Path)
	}
	return appverify.Snapshot{TreeSHA: snap.TreeSHA, BaseCommit: snap.BaseCommit, FileDigests: digests, Ignored: ignored}, nil
}

type finalizeVerifyAcceptance struct {
	runner appacceptance.Runner
	root   string
}

type finalizeAcceptanceFunc func(context.Context, appacceptance.EvaluateInput) (appacceptance.EvaluateOutput, error)

func (f finalizeAcceptanceFunc) Evaluate(ctx context.Context, in appacceptance.EvaluateInput) (appacceptance.EvaluateOutput, error) {
	return f(ctx, in)
}

func (a finalizeVerifyAcceptance) RunAcceptance(ctx context.Context, criteria []receipt.Acceptance) ([]appverify.AcceptanceResult, error) {
	out := make([]appverify.AcceptanceResult, 0, len(criteria))
	for _, c := range criteria {
		evaluated := appacceptance.Evaluate(ctx, a.runner, appacceptance.EvaluateInput{
			Criteria: []appacceptance.Criterion{{ID: c.ID, Command: c.Command, ExpectedKind: c.ExpectedKind}},
			WorkDir:  a.root,
		})
		if len(evaluated.Results) == 0 {
			continue
		}
		result := evaluated.Results[0]
		out = append(out, appverify.AcceptanceResult{ID: result.ID, Status: result.Status, ExitCode: result.ExitCode})
	}
	return out, nil
}

type finalizeVerifySignature struct{}

func (finalizeVerifySignature) Verify(envelope receipt.Envelope, trusted trust.TrustedKeys) error {
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

func mintTestReceipt(t *testing.T, root string, baseRef string) (appfinalize.Output, trust.TrustedKeys) {
	t.Helper()
	keyPath, trusted := newFinalizeSigningKey(t)
	out, err := appfinalize.Run(context.Background(),
		gateSnapshotter{git: git.Adapter{Root: root}},
		gateAcceptance{runner: process.Runner{}},
		sign.Ed25519Signer{PrivateKeyPath: keyPath},
		appfinalize.Input{
			TaskID:          "base-delta-seal",
			SessionID:       "base-delta-seal",
			Scope:           []string{"file.txt"},
			BaseRef:         baseRef,
			SpecFingerprint: "spec",
			Review:          passingReviewEvidence(t, root, []string{"file.txt"}, baseRef),
			HostUnderReview: receipt.HostUnderReview{Agent: "unknown"},
			Criteria:        []appacceptance.Criterion{{ID: "ac1", Command: "true", ExpectedKind: "exit_code_zero"}},
			WorkDir:         root,
			MintedAt:        time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != review.VerdictPass {
		t.Fatalf("finalize verdict = %q reason=%q", out.Verdict, out.Reason)
	}
	return out, trusted
}

func newFinalizeSigningKey(t *testing.T) (string, trust.TrustedKeys) {
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

func initFinalizeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	finalizeRunGit(t, root, "config", "user.name", "scafld")
	finalizeRunGit(t, root, "config", "user.email", "scafld@example.invalid")
	writeFinalizeFile(t, root, "file.txt", "before\n")
	finalizeRunGit(t, root, "add", "-A")
	finalizeRunGit(t, root, "commit", "-m", "base")
	return root
}

func finalizeRunGit(t *testing.T, root string, args ...string) {
	t.Helper()
	_ = finalizeGitOutput(t, root, args...)
}

func finalizeGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func writeFinalizeFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
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

func TestGateFindingsPreservesLine(t *testing.T) {
	t.Parallel()

	findings := gateFindings([]review.Finding{{
		ID:               "FINDING-001",
		Severity:         review.SeverityMedium,
		BlocksCompletion: true,
		Location:         &review.Location{Path: "internal/adapters/cli/finalize/run.go", Line: 742},
		Summary:          "line-specific finding",
		Validation:       "go test ./internal/adapters/cli/finalize -run TestGateFindingsPreservesLine -count=1",
	}})
	if len(findings) != 1 {
		t.Fatalf("findings = %+v", findings)
	}
	location, ok := findings[0]["location"].(map[string]any)
	if !ok {
		t.Fatalf("location = %#v, want structured location", findings[0]["location"])
	}
	if location["path"] != "internal/adapters/cli/finalize/run.go" || location["line"] != 742 {
		t.Fatalf("location = %+v, want path and line", location)
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
