package finalize

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	clioutput "github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	"github.com/nilstate/scafld/v2/internal/adapters/git"
	"github.com/nilstate/scafld/v2/internal/adapters/jsonstore"
	"github.com/nilstate/scafld/v2/internal/adapters/markdown"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	"github.com/nilstate/scafld/v2/internal/adapters/sign"
	"github.com/nilstate/scafld/v2/internal/adapters/trustcheck"
	appacceptance "github.com/nilstate/scafld/v2/internal/app/acceptance"
	"github.com/nilstate/scafld/v2/internal/app/envelope"
	appfinalize "github.com/nilstate/scafld/v2/internal/app/finalize"
	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/receipt"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/reviewscope"
	"github.com/nilstate/scafld/v2/internal/core/runartifact"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

const publicCommand = "finalize"

// Handler returns a CLI-compatible handler for the host-facing finalize command.
func Handler(stdin io.Reader) func(context.Context, []string, io.Writer, io.Writer) int {
	return func(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
		if err := Run(ctx, args, stdin, stdout); err != nil {
			exit := 2
			if clioutput.GateFailure(err) != nil {
				exit = 3
			}
			if hasJSONFlag(args) {
				return clioutput.EncodeEnvelope(stderr, envelope.Envelope[map[string]any]{
					OK:    false,
					Error: &envelope.Error{Code: clioutput.CodeName(exit), Message: err.Error(), Gate: clioutput.GateFailure(err), ExitCode: exit},
				}, exit)
			}
			fmt.Fprintf(stderr, "error: %v\n", err)
			if failure := clioutput.GateFailure(err); failure != nil {
				fmt.Fprint(stderr, clioutput.Gate(failure))
			}
			return exit
		}
		return 0
	}
}

func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

// Request is the finalize stdin payload sent by the MCP transport.
type Request struct {
	TaskID    string   `json:"task_id"`
	Root      string   `json:"root,omitempty"`
	BaseRef   string   `json:"base_ref,omitempty"`
	ScopeHint []string `json:"scope_hint,omitempty"`
}

// Run handles the public `scafld finalize <task_id>` command and the
// `scafld finalize --json --stdin` child process invoked by the finalize MCP
// transport. It composes deterministic snapshot, acceptance, and signing
// adapters around the accepted review evidence already sealed in the session
// ledger, then persists the signed receipt and archives the matching spec.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	req, err := readRequest(opts, stdin)
	if err != nil {
		return err
	}
	if strings.TrimSpace(req.TaskID) == "" {
		return errors.New("finalize requires a task_id in the request payload")
	}
	// Internal failures (bad config, missing spec, invalid ledger, compose error)
	// propagate as a non-zero exit so the MCP transport reports a tool error
	// instead of a successful call. Only a real gate verdict (a signed receipt or
	// fixable findings) is emitted as a structured success result.
	result, err := compose(ctx, req)
	if err != nil {
		return err
	}
	return emit(stdout, result, opts.JSON)
}

func compose(ctx context.Context, req Request) (map[string]any, error) {
	root := req.Root
	if strings.TrimSpace(root) == "" {
		root = "."
	}
	cfg, err := configadapter.LoadBase(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	specStore := markdown.Store{Root: root}
	model, hasSpec, err := loadGateModel(ctx, specStore, req)
	if err != nil {
		return nil, err
	}
	sessionStore := jsonstore.SessionStore{Root: root, TrustChecker: trustcheck.FromRoot(root, cfg.Verify.TrustedKeysPath)}
	now := time.Now().UTC()
	ledger, err := loadGateLedger(ctx, sessionStore, req.TaskID, now)
	if err != nil {
		return nil, err
	}
	// Refuse to extend a hash-chain that already failed replay, so a tampered or
	// corrupted prior receipt ledger cannot be silently continued.
	if !ledger.LedgerValid {
		return nil, fmt.Errorf("session ledger failed replay, refusing to extend the receipt chain: %s", ledger.LedgerError)
	}
	model = reconcile.FromSession(model, ledger)

	gitAdapter := git.Adapter{Root: root}
	baseRef, err := defaultFinalizeBaseRef(ctx, gitAdapter, req.BaseRef)
	if err != nil {
		return nil, err
	}
	req.BaseRef = baseRef
	authority := reviewgate.CurrentReviewGate(ledger)
	if terminal := reviewgate.TerminalAuthority(ledger); terminal.Valid && terminal.HasReceipt {
		authority = terminal
	}
	if authority.Valid && authority.HasReceipt {
		return finalizeWithArchive(ctx, root, req.TaskID, sessionStore, specStore, model, hasSpec, "", appfinalize.Output{
			Verdict: review.VerdictPass,
			Receipt: &authority.Receipt,
			Reason:  "recovered existing finalization receipt",
		}, now.Format(time.RFC3339))
	}
	scope, err := deriveGateScope(ctx, gitAdapter, model, req, hasSpec, ledger)
	if err != nil {
		return nil, err
	}
	state, err := projectFinalizeReviewGate(ctx, gitAdapter, model, ledger, scope, now)
	if err != nil {
		return nil, err
	}
	if state.Kind != reviewgate.KindReviewPassed {
		return nil, finalizeGateError(model, ledger, state)
	}
	authority = state.Authority
	if !authority.Valid || !authority.HasDossier && authority.ReviewEntry.Provider != "human" {
		return nil, errors.New("finalize review gate passed without usable accepted review evidence")
	}
	reviewSnapshot, err := (gateSnapshotter{git: gitAdapter}).Snapshot(ctx, appfinalize.SnapshotInput{Scope: scope, BaseRef: baseRef})
	if err != nil {
		return nil, fmt.Errorf("snapshot accepted review scope: %w", err)
	}
	provenance, ignored := snapshotReviewCoverage(reviewSnapshot)
	execCfg := configadapter.EffectiveExecution(root, cfg.Execution)
	diagnostics := runartifact.CommandDiagnosticsDir(root, req.TaskID)
	acceptanceRunner := process.Runner{DiagnosticsDir: diagnostics, DiagnosticName: "finalize-acceptance"}
	criteria := gateCriteria(model, ledger)
	if hasSpec && len(criteria) == 0 {
		return nil, errors.New("finalize requires at least one declared acceptance criterion for spec-backed work")
	}

	// The host vendor stamped into the receipt comes from genuine environment
	// markers only, never the self-declared SCAFLD_HOST_AGENT. Finalize does not
	// select or invoke a reviewer; it only carries the accepted review provider
	// from the ledger into the receipt.
	hostMarker := providers.DetectHostAgentMarker(os.Environ())
	if hostMarker == "" {
		hostMarker = "unknown"
	}
	keyPath, err := corebundle.HostPrivateKeyPath()
	if err != nil {
		return nil, fmt.Errorf("resolve signing key: %w", err)
	}

	input := appfinalize.Input{
		TaskID:    model.TaskID,
		SessionID: model.TaskID,
		Scope:     scope,
		BaseRef:   baseRef,
		Review: appfinalize.ReviewEvidence{
			Dossier:    authority.Dossier,
			Provenance: provenance,
			Ignored:    ignored,
			Reviewer: receipt.Reviewer{
				Provider: authority.ReviewEntry.Provider,
				Model:    authority.ReviewEntry.ProviderModel,
			},
		},
		SpecFingerprint: specFingerprint(model, scope),
		HostUnderReview: receipt.HostUnderReview{Agent: hostMarker, SessionID: model.TaskID},
		Independence: receipt.Independence{
			Level:    receipt.IndependenceLevelIsolationOnly,
			Distinct: false,
		},
		Criteria:        criteria,
		WorkDir:         root,
		Env:             execCfg.ProcessEnv(),
		Timeout:         execCfg.AbsoluteTimeout(),
		IdleTimeout:     execCfg.IdleTimeout(),
		PriorLedgerHead: ledger.LedgerHead,
		MintedAt:        now,
	}

	out, err := appfinalize.Run(ctx,
		gateSnapshotter{git: gitAdapter},
		gateAcceptance{runner: acceptanceRunner},
		sign.Ed25519Signer{PrivateKeyPath: keyPath},
		input,
	)
	if err != nil {
		return nil, err
	}
	return finalizeWithArchive(ctx, root, req.TaskID, sessionStore, specStore, model, hasSpec, "", out, now.Format(time.RFC3339))
}

type headResolver interface {
	ResolveHead(context.Context) (string, bool, error)
}

func defaultFinalizeBaseRef(ctx context.Context, resolver headResolver, requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested, nil
	}
	head, ok, err := resolver.ResolveHead(ctx)
	if err != nil {
		return "", fmt.Errorf("resolve HEAD for finalize base_ref: %w", err)
	}
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(head), nil
}

func projectFinalizeReviewGate(ctx context.Context, workspace git.Adapter, model spec.Model, ledger session.Session, scope []string, now time.Time) (reviewgate.State, error) {
	changed, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return reviewgate.State{}, fmt.Errorf("current workspace snapshot: %w", err)
	}
	head, hasHead, err := workspace.ResolveHead(ctx)
	if err != nil {
		return reviewgate.State{}, fmt.Errorf("resolve current HEAD: %w", err)
	}
	if !hasHead || strings.TrimSpace(head) == "" {
		head = "unborn"
	}
	comparison := reviewevidence.ComparisonSnapshot(changed)
	opts := reviewgate.Options{
		Now: now,
		WorkspaceSeal: reviewgate.WorkspaceSeal{
			Head:  strings.TrimSpace(head),
			Dirty: reviewevidence.SnapshotDirty(comparison),
			Diff:  reviewevidence.SnapshotDigest(comparison),
		},
		HasWorkspaceSeal: true,
	}
	authority := reviewgate.CurrentReviewGate(ledger)
	if authority.Valid && strings.TrimSpace(authority.ReviewEntry.ReviewedMaterialDigest) != "" && len(authority.ReviewEntry.ReviewedScope) > 0 {
		material, err := workspace.MaterialSeal(ctx, authority.ReviewEntry.ReviewedScope)
		if err != nil {
			return reviewgate.State{}, fmt.Errorf("current reviewed material seal: %w", err)
		}
		opts.MaterialSeal = material
		opts.HasMaterialSeal = true
	}
	return reviewgate.Project(ledger, model, opts), nil
}

func snapshotReviewCoverage(snap appfinalize.Snapshot) ([]receipt.Provenance, []string) {
	provenance := make([]receipt.Provenance, 0, len(snap.Files)+len(snap.Deleted))
	ignored := append([]string(nil), snap.IgnoredUnreviewed...)
	for _, file := range snap.Files {
		if file.Status == "gitlink" || blocklistedEvidence(file.Path) {
			ignored = append(ignored, file.Path)
			continue
		}
		provenance = append(provenance, receipt.Provenance{Kind: "evidence_file", Path: file.Path, SHA256: file.SHA256})
	}
	for _, path := range snap.Deleted {
		if blocklistedEvidence(path) {
			ignored = append(ignored, path)
			continue
		}
		provenance = append(provenance, receipt.Provenance{Kind: "deleted", Path: path})
	}
	return provenance, uniqueStrings(ignored)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func loadGateModel(ctx context.Context, store markdown.Store, req Request) (spec.Model, bool, error) {
	model, _, err := store.Load(ctx, req.TaskID)
	if err == nil {
		return model, true, nil
	}
	if !errors.Is(err, markdown.ErrSpecNotFound) {
		return spec.Model{}, false, fmt.Errorf("load spec: %w", err)
	}
	return spec.Model{
		TaskID:  req.TaskID,
		Title:   req.TaskID,
		Summary: "No hand-authored spec; scoped by gate request/diff.",
	}, false, nil
}

func loadGateLedger(ctx context.Context, store jsonstore.SessionStore, taskID string, now time.Time) (session.Session, error) {
	ledger, err := store.Load(ctx, taskID)
	if err == nil {
		return ledger, nil
	}
	if !errors.Is(err, jsonstore.ErrSessionNotFound) {
		return session.Session{}, fmt.Errorf("load session: %w", err)
	}
	return session.New(taskID, now.UTC().Format(time.RFC3339)), nil
}

type baseDiffPathser interface {
	BaseDiffPaths(context.Context, string) ([]string, error)
}

func deriveGateScope(ctx context.Context, diffs baseDiffPathser, model spec.Model, req Request, hasSpec bool, ledger session.Session) ([]string, error) {
	// Finalize must use the same material boundary the independent review sealed.
	// A sibling evidence root is provider context, not a path in this repository's
	// Git tree, so re-deriving scope from Markdown after review is unsafe.
	if scope := reviewscope.Literal(req.ScopeHint); len(scope) > 0 {
		return scope, nil
	}
	if scope := latestReviewedScope(model, ledger); len(scope) > 0 {
		return scope, nil
	}
	// Spec scope/touchpoints are the fallback only. The shared projection strips
	// prose and rejects paths that escape the repository root.
	scope := reviewscope.Derive(model, nil, nil)
	if len(scope) == 0 && strings.TrimSpace(req.BaseRef) != "" {
		paths, err := diffs.BaseDiffPaths(ctx, req.BaseRef)
		if err != nil {
			return nil, fmt.Errorf("derive base diff scope: %w", err)
		}
		// Base-diff paths are authoritative git paths, not spec prose, so they are
		// used literally. Prose-style scope filtering would drop top-level
		// extensionless changed files such as Makefile, Dockerfile, or LICENSE.
		scope = reviewscope.Literal(paths)
	}
	if len(scope) == 0 {
		return nil, errors.New("finalize scope is empty; provide scope_hint, a scoped spec, or a base_ref with changed paths")
	}
	return scope, nil
}

func latestReviewedScope(model spec.Model, ledger session.Session) []string {
	currentSpec := spec.ContractDigest(model)
	for index := len(ledger.Entries) - 1; index >= 0; index-- {
		entry := ledger.Entries[index]
		if entry.Type != "review" && entry.Type != "review_attempt" {
			continue
		}
		if strings.TrimSpace(entry.ReviewedSpec) != "" && entry.ReviewedSpec != currentSpec {
			continue
		}
		if scope := reviewscope.Literal(entry.ReviewedScope); len(scope) > 0 {
			return scope
		}
	}
	return nil
}

func finalize(ctx context.Context, root string, taskID string, sessions jsonstore.SessionStore, model spec.Model, hasSpec bool, out appfinalize.Output) (map[string]any, error) {
	return finalizeWithArchive(ctx, root, taskID, sessions, markdown.Store{}, model, hasSpec, "", out, time.Now().UTC().Format(time.RFC3339))
}

func finalizeWithArchive(ctx context.Context, root string, taskID string, sessions jsonstore.SessionStore, specs markdown.Store, model spec.Model, hasSpec bool, specPath string, out appfinalize.Output, now string) (map[string]any, error) {
	response := map[string]any{
		"ok":      true,
		"command": publicCommand,
		"tool":    publicCommand,
		"task_id": taskID,
		"verdict": out.Verdict,
	}
	if out.Receipt == nil {
		if err := appendAcceptanceEvidence(ctx, sessions, taskID, out.Acceptance.Results, phaseByCriterion(model), now); err != nil {
			return nil, err
		}
		response["findings"] = gateFindings(out.Findings)
		response["reason"] = out.Reason
		response["acceptance_passed"] = out.Acceptance.Passed
		if strings.TrimSpace(out.Independence.Level) != "" {
			response["independence"] = out.Independence
		}
		// Carry the per-criterion acceptance results so an acceptance failure (which
		// produces no reviewer findings) still tells the caller which command failed.
		response["acceptance"] = gateAcceptanceResults(out.Acceptance.Results)
		return response, nil
	}
	data, err := json.MarshalIndent(out.Receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode receipt: %w", err)
	}
	taskReceiptPath := filepath.Join(root, ".scafld", "receipts", taskID+".json")
	latestReceiptPath := filepath.Join(root, ".scafld", "receipts", "latest.json")
	digest, err := receipt.ReceiptDigest(out.Receipt.Body)
	if err != nil {
		return nil, err
	}
	now = out.Receipt.Body.MintedAt
	phaseByID := phaseByCriterion(model)
	if _, err := sessions.AppendTransaction(ctx, taskID, now, func(ledger session.Session) ([]session.Entry, error) {
		receiptSeen := false
		completeSeen := false
		for _, entry := range ledger.Entries {
			switch entry.Type {
			case session.EntryReceipt:
				if strings.TrimSpace(entry.ReceiptDigest) == digest {
					receiptSeen = true
				} else if strings.TrimSpace(entry.ReceiptDigest) != "" {
					return nil, fmt.Errorf("a different finalization receipt is already anchored")
				}
			case "complete":
				completeSeen = true
			}
		}
		if receiptSeen && completeSeen {
			return nil, nil
		}
		entries := make([]session.Entry, 0, len(out.Acceptance.Results)+2)
		if !receiptSeen {
			wantHead := session.NextLedgerHead(ledger.LedgerHead, digest)
			if strings.TrimSpace(out.Receipt.Body.LedgerHead) != wantHead {
				return nil, fmt.Errorf("receipt ledger_head %q does not chain from current ledger head %q", out.Receipt.Body.LedgerHead, ledger.LedgerHead)
			}
			entries = append(entries, acceptanceEvidenceEntries(out.Acceptance.Results, phaseByID)...)
			entries = append(entries, session.Entry{
				Type:          session.EntryReceipt,
				Status:        out.Verdict,
				Output:        string(data),
				ReceiptDigest: digest,
				LedgerHead:    out.Receipt.Body.LedgerHead,
			})
		}
		if !completeSeen {
			entries = append(entries, session.Entry{Type: "complete", Status: "completed", Reason: "finalization receipt passed"})
		}
		return entries, nil
	}); err != nil {
		return nil, fmt.Errorf("anchor finalization: %w", err)
	}
	// Receipt files are written only after the receipt is anchored in the ledger
	// and the task is marked complete, so a losing concurrent finalize cannot
	// leave behind an unanchored task receipt.
	if err := os.MkdirAll(filepath.Dir(taskReceiptPath), 0o755); err != nil {
		return nil, fmt.Errorf("create receipts dir: %w", err)
	}
	if err := atomicfile.Write(taskReceiptPath, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("write receipt: %w", err)
	}
	if err := atomicfile.Write(latestReceiptPath, append(data, '\n'), 0o644); err != nil {
		return nil, fmt.Errorf("write latest receipt: %w", err)
	}
	if hasSpec {
		if strings.TrimSpace(specPath) == "" {
			var err error
			specPath, err = specs.Find(taskID)
			if err != nil {
				return nil, fmt.Errorf("find spec for archive: %w", err)
			}
		}
		ledger, err := sessions.Load(ctx, taskID)
		if err != nil {
			return nil, fmt.Errorf("reload completed ledger: %w", err)
		}
		completed := reconcile.FromSession(model, ledger)
		completed.Status = spec.StatusCompleted
		completed.Updated = now
		completed.CurrentState.Next = "none"
		completed.CurrentState.AllowedFollowUp = "none"
		if err := specs.Save(ctx, specPath, completed); err != nil {
			return nil, fmt.Errorf("archive completed spec: %w", err)
		}
		response["spec_archived"] = true
	}
	// Return the signed receipt itself, not just a path, so finalize satisfies
	// the single-call contract: the MCP/JSON response is the receipt artifact.
	response["receipt"] = out.Receipt
	response["receipt_path"] = latestReceiptPath
	response["task_receipt_path"] = taskReceiptPath
	response["ledger_head"] = out.Receipt.Body.LedgerHead
	response["independence"] = out.Receipt.Body.Independence
	return response, nil
}

func acceptanceEvidenceEntries(results []appacceptance.CriterionResult, phaseByID map[string]string) []session.Entry {
	entries := make([]session.Entry, 0, len(results))
	for _, result := range results {
		if strings.TrimSpace(result.ID) == "" {
			continue
		}
		entries = append(entries, session.Entry{
			Type:           "criterion",
			CriterionID:    result.ID,
			PhaseID:        phaseByID[result.ID],
			Status:         result.Status,
			Reason:         result.Reason,
			Command:        result.Command,
			ExpectedKind:   result.ExpectedKind,
			CriterionType:  resultCriterionType(result),
			ExitCode:       result.ExitCode,
			Output:         result.Evidence,
			Path:           result.DiagnosticPath,
			EvidenceDigest: result.EvidenceDigest,
			EvidenceActor:  result.EvidenceActor,
		})
	}
	return entries
}

func appendAcceptanceEvidence(ctx context.Context, sessions jsonstore.SessionStore, taskID string, results []appacceptance.CriterionResult, phaseByID map[string]string, now string) error {
	for _, entry := range acceptanceEvidenceEntries(results, phaseByID) {
		if _, err := sessions.Append(ctx, taskID, entry, now); err != nil {
			return fmt.Errorf("append finalization acceptance evidence: %w", err)
		}
	}
	return nil
}

func resultCriterionType(result appacceptance.CriterionResult) string {
	value := strings.TrimSpace(result.Type)
	if value == "" {
		return "command"
	}
	return value
}

func phaseByCriterion(model spec.Model) map[string]string {
	out := map[string]string{}
	for _, phase := range model.Phases {
		for _, criterion := range phase.Acceptance {
			if strings.TrimSpace(criterion.ID) != "" {
				phaseID := strings.TrimSpace(criterion.PhaseID)
				if phaseID == "" {
					phaseID = phase.ID
				}
				out[criterion.ID] = phaseID
			}
		}
	}
	for _, criterion := range model.Acceptance.Criteria {
		if strings.TrimSpace(criterion.ID) != "" && strings.TrimSpace(criterion.PhaseID) != "" {
			out[criterion.ID] = criterion.PhaseID
		}
	}
	return out
}

// gateSnapshotter adapts the git snapshot to the app/gate snapshotter port.
type gateSnapshotter struct{ git git.Adapter }

func (s gateSnapshotter) Snapshot(ctx context.Context, in appfinalize.SnapshotInput) (appfinalize.Snapshot, error) {
	snap, err := s.git.Snapshot(ctx, git.SnapshotInput{Scope: in.Scope, BaseRef: in.BaseRef})
	if err != nil {
		return appfinalize.Snapshot{}, err
	}
	digests := make(map[string]string, len(snap.FileDigests))
	files := make([]appfinalize.FileFact, 0, len(snap.FileDigests))
	for _, d := range snap.FileDigests {
		digests[d.Path] = d.SHA256
		files = append(files, appfinalize.FileFact{Path: d.Path, Status: d.Status, SHA256: d.SHA256})
	}
	ignored := make([]string, 0, len(snap.IgnoredUnreviewed))
	for _, ig := range snap.IgnoredUnreviewed {
		ignored = append(ignored, ig.Path)
	}
	deleted := make([]string, 0, len(snap.DeletedPaths))
	for _, d := range snap.DeletedPaths {
		deleted = append(deleted, d.Path)
	}
	return appfinalize.Snapshot{
		TreeSHA:           snap.TreeSHA,
		BaseCommit:        snap.BaseCommit,
		HeadCommit:        snap.HeadCommit,
		FileDigests:       digests,
		Files:             files,
		IgnoredUnreviewed: ignored,
		Deleted:           deleted,
	}, nil
}

// gateAcceptance adapts the shared acceptance engine to the app/gate port.
type gateAcceptance struct{ runner appacceptance.Runner }

func (a gateAcceptance) Evaluate(ctx context.Context, in appacceptance.EvaluateInput) (appacceptance.EvaluateOutput, error) {
	return appacceptance.Evaluate(ctx, a.runner, in), nil
}

func blocklistedEvidence(path string) bool {
	switch filepath.Base(filepath.FromSlash(path)) {
	case "CLAUDE.md", "AGENTS.md", "GEMINI.md":
		return true
	}
	return strings.TrimSpace(path) == ".scafld/config.yaml"
}

func gateCriteria(model spec.Model, ledger session.Session) []appacceptance.Criterion {
	criteria := model.AllCriteria()
	out := make([]appacceptance.Criterion, 0, len(criteria))
	for _, c := range criteria {
		criterion := appacceptance.Criterion{
			ID:           c.ID,
			Type:         c.Type,
			Command:      c.Command,
			ExpectedKind: string(c.ExpectedKind),
		}
		if c.Type == "manual" && c.ExpectedKind == acceptance.ExpectedManual {
			if entry, ok := session.LatestManualEvidence(ledger, c.ID, c.PhaseID, string(c.ExpectedKind), c.Type); ok {
				criterion.ManualEvidence = &appacceptance.ManualEvidence{
					Disposition:    entry.Status,
					EvidenceDigest: entry.EvidenceDigest,
					Actor:          entry.EvidenceActor,
					RecordedAt:     entry.RecordedAt,
					Reason:         entry.Reason,
				}
			}
		}
		out = append(out, criterion)
	}
	return out
}

func finalizeGateError(model spec.Model, ledger session.Session, state reviewgate.State) error {
	reason := firstNonBlank(state.Reason, model.CurrentState.Reason, "finalize requires a passing review gate")
	actual := firstNonBlank(state.Actual, "task status "+string(model.Status))
	blockers := append([]string(nil), state.Blockers...)
	evidence := append([]string(nil), state.Evidence...)
	if len(blockers) == 0 {
		blockers, evidence = finalizeAcceptanceBlockers(model, ledger)
	}
	if len(blockers) == 0 {
		blockers = []string{reason}
	}
	next := firstNonBlank(state.Next, model.CurrentState.AllowedFollowUp, "scafld handoff "+model.TaskID)
	return gate.New(errors.New("finalize review gate has not passed"), gate.Failure{
		Gate:     "finalize",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: evidence,
		Expected: "all acceptance criteria pass and an accepted review gate is current",
		Actual:   actual,
		Blockers: blockers,
		Next:     next,
	})
}

func finalizeAcceptanceBlockers(model spec.Model, ledger session.Session) ([]string, []string) {
	currentPhase := strings.TrimSpace(model.CurrentState.CurrentPhase)
	var blockers []string
	var evidence []string
	for _, criterion := range model.AllCriteria() {
		if currentPhase != "" && currentPhase != "final" && criterion.PhaseID != currentPhase {
			continue
		}
		if currentPhase == "final" && criterion.PhaseID != "" {
			continue
		}
		if criterion.Status == "pass" {
			continue
		}
		label := criterion.ID
		if strings.TrimSpace(criterion.Title) != "" {
			label += ": " + strings.TrimSpace(criterion.Title)
		}
		if criterion.Type == "manual" && criterion.ExpectedKind == acceptance.ExpectedManual {
			label += ": manual evidence required; run scafld handoff " + model.TaskID
		} else if strings.TrimSpace(criterion.Evidence) != "" {
			label += ": " + strings.TrimSpace(criterion.Evidence)
		}
		blockers = append(blockers, label)
		if entry, ok := session.LatestCriterionEntry(ledger, criterion.ID); ok {
			if entry.Path != "" {
				evidence = append(evidence, entry.Path)
			} else if entry.ID != "" {
				evidence = append(evidence, entry.ID)
			}
		}
	}
	return blockers, evidence
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func specFingerprint(model spec.Model, scope []string) string {
	type criterion struct {
		ID           string `json:"id"`
		Command      string `json:"command"`
		ExpectedKind string `json:"expected_kind"`
	}
	var criteria []criterion
	for _, c := range model.AllCriteria() {
		criteria = append(criteria, criterion{ID: c.ID, Command: c.Command, ExpectedKind: string(c.ExpectedKind)})
	}
	sort.Slice(criteria, func(i, j int) bool { return criteria[i].ID < criteria[j].ID })
	sortedScope := append([]string(nil), scope...)
	sort.Strings(sortedScope)
	// Bind the stable approved contract (not just scope/criteria) so two specs
	// with the same scope/criteria but different title/summary/objectives do not
	// collide on one fingerprint. Mutable execution state (status, review, current
	// state) is excluded so the fingerprint stays stable across a task's lifecycle.
	payload, _ := json.Marshal(map[string]any{
		"task_id":      model.TaskID,
		"title":        model.Title,
		"summary":      model.Summary,
		"objectives":   model.Objectives,
		"scope":        sortedScope,
		"touchpoints":  model.Touchpoints,
		"dependencies": model.Dependencies,
		"assumptions":  model.Assumptions,
		"invariants":   model.Context.Invariants,
		"criteria":     criteria,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func gateFindings(findings []review.Finding) []map[string]any {
	out := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		item := map[string]any{
			"id":       f.ID,
			"severity": string(f.Severity),
			"summary":  f.Summary,
			"blocks":   f.BlocksCompletion,
		}
		if f.Location != nil {
			location := map[string]any{"path": f.Location.Path}
			if f.Location.Line > 0 {
				location["line"] = f.Location.Line
			}
			item["location"] = location
		}
		if strings.TrimSpace(f.Validation) != "" {
			item["validation"] = f.Validation
		}
		out = append(out, item)
	}
	return out
}

func gateAcceptanceResults(results []appacceptance.CriterionResult) []map[string]any {
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		item := map[string]any{
			"id":        r.ID,
			"command":   r.Command,
			"status":    r.Status,
			"exit_code": r.ExitCode,
		}
		if strings.TrimSpace(r.Reason) != "" {
			item["reason"] = r.Reason
		}
		if strings.TrimSpace(r.DiagnosticPath) != "" {
			item["diagnostic"] = r.DiagnosticPath
		}
		out = append(out, item)
	}
	return out
}

func readRequest(opts options, stdin io.Reader) (Request, error) {
	req := Request{
		TaskID:    opts.TaskID,
		Root:      opts.Root,
		BaseRef:   opts.BaseRef,
		ScopeHint: append([]string(nil), opts.ScopeHint...),
	}
	if !opts.Stdin {
		return req, nil
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return Request{}, fmt.Errorf("read finalize stdin: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return req, nil
	}
	var payload Request
	if err := json.Unmarshal(data, &payload); err != nil {
		return Request{}, fmt.Errorf("parse finalize stdin JSON: %w", err)
	}
	req, err = mergeRequestOptions(payload, opts)
	if err != nil {
		return Request{}, err
	}
	return req, nil
}

func mergeRequestOptions(req Request, opts options) (Request, error) {
	if opts.TaskID != "" {
		if req.TaskID != "" && req.TaskID != opts.TaskID {
			return Request{}, fmt.Errorf("finalize task_id %q conflicts with stdin task_id %q", opts.TaskID, req.TaskID)
		}
		req.TaskID = opts.TaskID
	}
	if opts.Root != "" {
		req.Root = opts.Root
	}
	if opts.BaseRef != "" {
		req.BaseRef = opts.BaseRef
	}
	if len(opts.ScopeHint) > 0 {
		req.ScopeHint = append([]string(nil), opts.ScopeHint...)
	}
	return req, nil
}

func emit(stdout io.Writer, payload map[string]any, asJSON bool) error {
	if !asJSON {
		return emitText(stdout, payload)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, string(data))
	return err
}

func emitText(stdout io.Writer, payload map[string]any) error {
	taskID, _ := payload["task_id"].(string)
	verdict, _ := payload["verdict"].(string)
	if taskID == "" {
		taskID = "work"
	}
	if verdict == "pass" {
		fmt.Fprintf(stdout, "finalize passed: %s\n", taskID)
		if path, _ := payload["task_receipt_path"].(string); path != "" {
			fmt.Fprintf(stdout, "receipt: %s\n", path)
		} else if path, _ := payload["receipt_path"].(string); path != "" {
			fmt.Fprintf(stdout, "receipt: %s\n", path)
		}
		return nil
	}
	fmt.Fprintf(stdout, "finalize blocked: %s\n", taskID)
	if reason, _ := payload["reason"].(string); reason != "" {
		fmt.Fprintf(stdout, "reason: %s\n", reason)
	}
	if findings, ok := payload["findings"].([]map[string]any); ok {
		for _, finding := range findings {
			summary, _ := finding["summary"].(string)
			if summary != "" {
				fmt.Fprintf(stdout, "- %s\n", summary)
			}
		}
	}
	return nil
}

type options struct {
	JSON      bool
	Stdin     bool
	TaskID    string
	Root      string
	BaseRef   string
	ScopeHint []string
}

func parseOptions(args []string) (options, error) {
	var opts options
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			opts.JSON = true
		case "--stdin":
			opts.Stdin = true
		case "--root":
			value, err := nextFinalizeArg(args, &i, arg)
			if err != nil {
				return options{}, err
			}
			opts.Root = value
		case "--base-ref":
			value, err := nextFinalizeArg(args, &i, arg)
			if err != nil {
				return options{}, err
			}
			opts.BaseRef = value
		case "--scope-hint":
			value, err := nextFinalizeArg(args, &i, arg)
			if err != nil {
				return options{}, err
			}
			opts.ScopeHint = append(opts.ScopeHint, value)
		default:
			if key, value, ok := strings.Cut(strings.TrimPrefix(arg, "--"), "="); ok && strings.HasPrefix(arg, "--") {
				switch key {
				case "root":
					opts.Root = value
				case "base-ref":
					opts.BaseRef = value
				case "scope-hint":
					opts.ScopeHint = append(opts.ScopeHint, value)
				default:
					return options{}, fmt.Errorf("unknown finalize argument %q", arg)
				}
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return options{}, fmt.Errorf("unknown finalize argument %q", arg)
			}
			if opts.TaskID != "" {
				return options{}, fmt.Errorf("finalize accepts at most one task_id, got %q and %q", opts.TaskID, arg)
			}
			opts.TaskID = arg
		}
	}
	return opts, nil
}

func nextFinalizeArg(args []string, index *int, flag string) (string, error) {
	if *index+1 >= len(args) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	*index = *index + 1
	return args[*index], nil
}
