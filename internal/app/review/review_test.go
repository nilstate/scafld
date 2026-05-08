package review

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
	"github.com/nilstate/scafld/v2/internal/testkit/providerfake"
)

type fakeSpecs struct{ model spec.Model }

func (f *fakeSpecs) Load(context.Context, string) (spec.Model, string, error) {
	return f.model, "task.md", nil
}
func (f *fakeSpecs) Save(_ context.Context, _ string, model spec.Model) error {
	f.model = model
	return nil
}

type fakeSessions struct{ ledger session.Session }

func (f *fakeSessions) Append(_ context.Context, taskID string, entry session.Entry, now string) (session.Session, error) {
	if f.ledger.TaskID == "" {
		f.ledger = session.New(taskID, now)
	}
	if entry.RecordedAt == "" {
		entry.RecordedAt = now
	}
	f.ledger = f.ledger.WithEntry(entry)
	return f.ledger, nil
}

func (f *fakeSessions) Load(context.Context, string) (session.Session, error) { return f.ledger, nil }

type fakeProvider struct{ packet corereview.Packet }

func (f fakeProvider) Invoke(context.Context, corereview.Request) (corereview.Packet, error) {
	return f.packet, nil
}

type promptProvider struct {
	req    corereview.Request
	packet corereview.Packet
}

func (p *promptProvider) Invoke(_ context.Context, req corereview.Request) (corereview.Packet, error) {
	p.req = req
	return p.packet, nil
}

type fakeWorkspace struct {
	snapshots [][]string
	calls     int
}

func (f *fakeWorkspace) ChangedFiles(context.Context) ([]string, error) {
	if f.calls >= len(f.snapshots) {
		return nil, nil
	}
	files := f.snapshots[f.calls]
	f.calls++
	return files, nil
}

type fakeClock struct{}

func (fakeClock) Now() time.Time { return time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC) }

func TestProviderVerdictDrivesReviewState(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, nil, fakeProvider{packet: corereview.Packet{
		Verdict:  "fail",
		Findings: []corereview.Finding{{ID: "f1", Severity: corereview.SeverityBlocking, Summary: "bug"}},
	}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != "fail" || len(out.Findings) != 1 {
		t.Fatalf("output = %+v", out)
	}
	if out.Repair == nil || out.Repair.Gate != "review" || out.Repair.Next != "scafld handoff task" || len(out.Repair.Blockers) != 1 {
		t.Fatalf("repair = %+v", out.Repair)
	}
	if specs.model.CurrentState.AllowedFollowUp != "scafld handoff task" {
		t.Fatalf("next action = %q", specs.model.CurrentState.AllowedFollowUp)
	}
	if len(sessions.ledger.Entries) != 2 ||
		sessions.ledger.Entries[0].Type != "review_attempt" ||
		sessions.ledger.Entries[0].Status != "running" ||
		sessions.ledger.Entries[1].Type != "review" ||
		!strings.Contains(sessions.ledger.Entries[1].Output, "bug") {
		t.Fatalf("review findings were not recorded in session: %+v", sessions.ledger.Entries)
	}
}

func TestReviewRecordsRunningAttemptBeforeProviderReturns(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	sessions := &fakeSessions{}
	provider := providerFunc(func(context.Context, corereview.Request) (corereview.Packet, error) {
		if len(sessions.ledger.Entries) != 1 ||
			sessions.ledger.Entries[0].Type != "review_attempt" ||
			sessions.ledger.Entries[0].Status != "running" {
			t.Fatalf("review attempt was not recorded before provider invocation: %+v", sessions.ledger.Entries)
		}
		return corereview.Packet{Verdict: corereview.VerdictPass}, nil
	})
	if _, err := Run(context.Background(), specs, sessions, nil, provider, fakeClock{}, "task"); err != nil {
		t.Fatal(err)
	}
}

type providerFunc func(context.Context, corereview.Request) (corereview.Packet, error)

func (f providerFunc) Invoke(ctx context.Context, req corereview.Request) (corereview.Packet, error) {
	return f(ctx, req)
}

func TestProviderTimeoutMutationInvalidOutputPacketRepairFindingSignal(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	sessions := &fakeSessions{}
	out, err := Run(context.Background(), specs, sessions, nil, providerfake.Provider{Mode: providerfake.ModeMutation}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != "fail" {
		t.Fatalf("mutation should fail review: %+v", out)
	}
	if len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("mutation finding = %+v", out.Findings)
	}
}

func TestReviewRejectsInvalidDirectProviderPacket(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeProvider{packet: corereview.Packet{Verdict: "maybe"}}, fakeClock{}, "task")
	if !errors.Is(err, corereview.ErrInvalidPacket) {
		t.Fatalf("invalid provider packet err = %v", err)
	}
}

func TestReviewRejectsTaskBeforeBuildReviewState(t *testing.T) {
	t.Parallel()

	for _, status := range []spec.Status{spec.StatusDraft, spec.StatusApproved, spec.StatusBlocked} {
		specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: status}}
		_, err := Run(context.Background(), specs, &fakeSessions{}, nil, providerFunc(func(context.Context, corereview.Request) (corereview.Packet, error) {
			t.Fatalf("provider should not run for status %s", status)
			return corereview.Packet{}, nil
		}), fakeClock{}, "task")
		if !errors.Is(err, ErrSpecNotReviewable) {
			t.Fatalf("status %s err = %v, want %v", status, err, ErrSpecNotReviewable)
		}
	}
}

func TestHumanReviewedRecordsAuditedPassingReviewWithoutProvider(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	sessions := &fakeSessions{}
	provider := providerFunc(func(context.Context, corereview.Request) (corereview.Packet, error) {
		t.Fatal("provider should not run for human-reviewed review")
		return corereview.Packet{}, nil
	})
	out, err := RunWithInput(context.Background(), specs, sessions, nil, provider, fakeClock{}, Input{
		TaskID:        "task",
		HumanReviewed: true,
		Reason:        "operator reviewed PR 123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictPass || out.Next != "scafld complete task" {
		t.Fatalf("output = %+v", out)
	}
	if len(sessions.ledger.Entries) != 2 ||
		sessions.ledger.Entries[0].Type != "review_override" ||
		sessions.ledger.Entries[0].Provider != "human" ||
		sessions.ledger.Entries[1].Type != "review" ||
		sessions.ledger.Entries[1].Status != corereview.VerdictPass ||
		sessions.ledger.Entries[1].Provider != "human" {
		t.Fatalf("human review evidence = %+v", sessions.ledger.Entries)
	}
	if specs.model.Review.Verdict != corereview.VerdictPass || specs.model.CurrentState.AllowedFollowUp != "scafld complete task" {
		t.Fatalf("projected model = %+v", specs.model)
	}
}

func TestHumanReviewedRequiresReason(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	_, err := RunWithInput(context.Background(), specs, &fakeSessions{}, nil, fakeProvider{}, fakeClock{}, Input{TaskID: "task", HumanReviewed: true})
	if err == nil || !strings.Contains(err.Error(), "--reason") {
		t.Fatalf("error = %v, want reason requirement", err)
	}
}

func TestReviewPromptCarriesTaskContractToProvider(t *testing.T) {
	t.Parallel()

	provider := &promptProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}
	workspace := &fakeWorkspace{snapshots: [][]string{{" M root-hash README.md", " M api-hash api/handler.go"}, {" M api-hash api/handler.go"}}}
	specs := &fakeSpecs{model: spec.Model{
		TaskID:     "task",
		Title:      "Task",
		Status:     spec.StatusReview,
		Summary:    "Review this",
		Objectives: []string{"Keep evidence"},
		Context: spec.Context{
			Packages:      []string{"api"},
			FilesImpacted: []string{"api/handler.go"},
			Invariants:    []string{"tenant_isolation"},
		},
		Scope:       []string{"Only API contract changes"},
		Touchpoints: []string{"MCP API"},
		Acceptance:  spec.Acceptance{Criteria: []spec.Criterion{{ID: "ac1", Command: "go test ./...", ExpectedKind: "exit_code_zero", Status: "pass", Evidence: "exit code was 0"}}},
		Phases:      []spec.Phase{{ID: "phase1", Name: "Implementation", Changes: []string{"Update API prompt context"}}},
	}}
	_, err := RunWithInput(context.Background(), specs, &fakeSessions{}, workspace, provider, fakeClock{}, Input{
		TaskID:      "task",
		ReviewScope: []string{"api"},
		Passes: []Pass{{
			ID:          "regression_hunt",
			Category:    "adversarial",
			Order:       30,
			Title:       "Regression Hunt",
			Description: "Trace downstream consumers",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if provider.req.TaskID != "task" || !strings.Contains(provider.req.Prompt, "Review this") || !strings.Contains(provider.req.Prompt, "ac1") {
		t.Fatalf("provider request = %+v", provider.req)
	}
	if !strings.Contains(provider.req.Prompt, "Evidence: exit code was 0") || !strings.Contains(provider.req.Prompt, "Do not run build, test, or mutation commands") {
		t.Fatalf("provider request = %+v", provider.req)
	}
	if !strings.Contains(provider.req.Prompt, "## Declared Invariants") || !strings.Contains(provider.req.Prompt, "tenant_isolation") {
		t.Fatalf("provider request missing declared invariants = %+v", provider.req)
	}
	if !strings.Contains(provider.req.Prompt, "## Review Focus") || !strings.Contains(provider.req.Prompt, "adversarial: Regression Hunt") {
		t.Fatalf("provider request missing configured review focus = %+v", provider.req)
	}
	for _, want := range []string{"## Task Scope", "Explicit review scope", "`api`", "`api/handler.go`", "MCP API", "Implementation changes", "## Workspace Baseline Before Review", "`api/handler.go`", "unchanged dirty paths from the approval baseline are context"} {
		if !strings.Contains(provider.req.Prompt, want) {
			t.Fatalf("provider request missing %q:\n%s", want, provider.req.Prompt)
		}
	}
	if strings.Contains(provider.req.Prompt, "README.md") {
		t.Fatalf("review-scope leaked unrelated baseline dirt into prompt:\n%s", provider.req.Prompt)
	}
}

func TestWorkspaceMutationGuardOverridesCleanProviderPacket(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	workspace := &fakeWorkspace{snapshots: [][]string{{"?? hash-a existing"}, {"?? hash-b existing"}}}
	out, err := Run(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("mutation guard output = %+v", out)
	}
	if !strings.Contains(out.Findings[0].Summary, "workspace changed during review") || !strings.Contains(out.Findings[0].Summary, "changed existing") {
		t.Fatalf("mutation guard summary = %q", out.Findings[0].Summary)
	}
}

func TestWorkspaceMutationGuardPreservesProviderFindings(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	workspace := &fakeWorkspace{snapshots: [][]string{{}, {"?? hash-new added"}}}
	providerPacket := corereview.Packet{
		Verdict:  corereview.VerdictFail,
		Findings: []corereview.Finding{{ID: "provider-finding", Severity: corereview.SeverityBlocking, Summary: "provider bug"}},
	}
	out, err := Run(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: providerPacket}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 2 {
		t.Fatalf("mutation guard output = %+v", out)
	}
	if out.Findings[0].ID != "provider-finding" || out.Findings[1].ID != "workspace_mutation" {
		t.Fatalf("findings should preserve provider finding then append guard finding: %+v", out.Findings)
	}
}

func TestReviewScopeDoesNotLimitMutationGuard(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	workspace := &fakeWorkspace{snapshots: [][]string{
		{" M root-old README.md", " M api-old api/handler.go"},
		{" M root-new README.md", " M api-old api/handler.go"},
	}}
	out, err := RunWithInput(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, Input{TaskID: "task", ReviewScope: []string{"api"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("outside-scope mutation during review should fail read-only guard: %+v", out)
	}

	workspace = &fakeWorkspace{snapshots: [][]string{
		{" M api-old api/handler.go"},
		{" M api-new api/handler.go"},
	}}
	out, err = RunWithInput(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, Input{TaskID: "task", ReviewScope: []string{"api"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("inside-scope mutation should fail scoped review: %+v", out)
	}
}

func TestWorkspaceMutationGuardSeesSpecMutation(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	workspace := &fakeWorkspace{snapshots: [][]string{
		{" M spec-old .scafld/specs/active/task.md"},
		{" M spec-new .scafld/specs/active/task.md"},
	}}
	out, err := Run(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("spec mutation during review should fail read-only guard: %+v", out)
	}
}

func TestReviewComparisonIgnoresManagedSpecProjection(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{
		Type:   session.EntryWorkspaceBaseline,
		Status: "captured",
		Output: " M spec-old .scafld/specs/active/task.md",
	})
	sessions := &fakeSessions{ledger: ledger}
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Status: spec.StatusReview}}
	workspace := &fakeWorkspace{snapshots: [][]string{
		{" M spec-new .scafld/specs/active/task.md"},
		{" M spec-new .scafld/specs/active/task.md"},
	}}
	out, err := Run(context.Background(), specs, sessions, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictPass {
		t.Fatalf("managed spec projection should not be scope drift: %+v", out)
	}
}

func TestReviewUsesApprovalBaselineForScopeDrift(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{
		Type:   session.EntryWorkspaceBaseline,
		Status: "captured",
		Output: " M root-old README.md\n M api-old api/handler.go",
	})
	sessions := &fakeSessions{ledger: ledger}
	specs := &fakeSpecs{model: spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusReview,
		Context: spec.Context{
			FilesImpacted: []string{"`api/handler.go`"},
		},
	}}
	workspace := &fakeWorkspace{snapshots: [][]string{
		{" M root-old README.md", " M api-new api/handler.go", " M docs-new docs/index.md"},
		{" M root-old README.md", " M api-new api/handler.go", " M docs-new docs/index.md"},
	}}
	out, err := RunWithInput(context.Background(), specs, sessions, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "scope_drift" {
		t.Fatalf("scope drift should fail review: %+v", out)
	}
	if !strings.Contains(out.Findings[0].Summary, "docs/index.md") || strings.Contains(out.Findings[0].Summary, "README.md") {
		t.Fatalf("scope drift should cite new outside-scope drift only: %+v", out.Findings)
	}
}

func TestReviewDerivesScopeFromTouchpointsAndScopeBullets(t *testing.T) {
	t.Parallel()

	ledger := session.New("task", "now")
	ledger = ledger.WithEntry(session.Entry{
		Type:   session.EntryWorkspaceBaseline,
		Status: "captured",
		Output: " M old internal/app/report/report.go\n M old docs/review.md\n M old README.md",
	})
	sessions := &fakeSessions{ledger: ledger}
	specs := &fakeSpecs{model: spec.Model{
		TaskID: "task",
		Title:  "Task",
		Status: spec.StatusReview,
		Scope:  []string{"Documentation under `docs/**`."},
		Touchpoints: []string{
			"`internal/app/report`",
			"`docs/**`",
		},
	}}
	workspace := &fakeWorkspace{snapshots: [][]string{
		{" M new internal/app/report/report.go", " M new docs/review.md", " M new README.md"},
		{" M new internal/app/report/report.go", " M new docs/review.md", " M new README.md"},
	}}
	out, err := RunWithInput(context.Background(), specs, sessions, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, Input{TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "scope_drift" {
		t.Fatalf("outside touchpoint should fail review: %+v", out)
	}
	if strings.Contains(out.Findings[0].Summary, "internal/app/report") || strings.Contains(out.Findings[0].Summary, "docs/review.md") || !strings.Contains(out.Findings[0].Summary, "README.md") {
		t.Fatalf("scope drift should only cite outside-scope README change: %+v", out.Findings)
	}
}

func TestWorkspaceMutationsDetectsAddModifyAndDelete(t *testing.T) {
	t.Parallel()

	mutated := coreworkspace.MutationStrings(coreworkspace.Diff(
		[]string{" M hash-a same", " M hash-old modified", " D deleted removed"},
		[]string{" M hash-a same", " M hash-new modified", "?? hash-add added"},
	))
	if len(mutated) != 3 {
		t.Fatalf("mutations = %+v", mutated)
	}
	for _, want := range []string{"added added", "changed modified", "removed removed"} {
		if !strings.Contains(strings.Join(mutated, "\n"), want) {
			t.Fatalf("mutations missing %q: %+v", want, mutated)
		}
	}
}
