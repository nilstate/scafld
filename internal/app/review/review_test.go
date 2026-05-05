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

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task"}}
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
	if specs.model.CurrentState.AllowedFollowUp != "scafld handoff task" {
		t.Fatalf("next action = %q", specs.model.CurrentState.AllowedFollowUp)
	}
	if len(sessions.ledger.Entries) != 1 || !strings.Contains(sessions.ledger.Entries[0].Output, "bug") {
		t.Fatalf("review findings were not recorded in session: %+v", sessions.ledger.Entries)
	}
}

func TestProviderTimeoutMutationInvalidOutputPacketRepairFindingSignal(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task"}}
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

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task"}}
	_, err := Run(context.Background(), specs, &fakeSessions{}, nil, fakeProvider{packet: corereview.Packet{Verdict: "maybe"}}, fakeClock{}, "task")
	if !errors.Is(err, corereview.ErrInvalidPacket) {
		t.Fatalf("invalid provider packet err = %v", err)
	}
}

func TestReviewPromptCarriesTaskContractToProvider(t *testing.T) {
	t.Parallel()

	provider := &promptProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}
	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task", Summary: "Review this", Objectives: []string{"Keep evidence"}, Acceptance: spec.Acceptance{Criteria: []spec.Criterion{{ID: "ac1", Command: "go test ./...", ExpectedKind: "exit_code_zero", Status: "pass", Evidence: "exit code was 0"}}}}}
	_, err := RunWithInput(context.Background(), specs, &fakeSessions{}, nil, provider, fakeClock{}, Input{
		TaskID: "task",
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
	if !strings.Contains(provider.req.Prompt, "## Review Focus") || !strings.Contains(provider.req.Prompt, "adversarial: Regression Hunt") {
		t.Fatalf("provider request missing configured review focus = %+v", provider.req)
	}
}

func TestWorkspaceMutationGuardOverridesCleanProviderPacket(t *testing.T) {
	t.Parallel()

	specs := &fakeSpecs{model: spec.Model{TaskID: "task", Title: "Task"}}
	workspace := &fakeWorkspace{snapshots: [][]string{{"hash-a existing"}, {"hash-b existing"}}}
	out, err := Run(context.Background(), specs, &fakeSessions{}, workspace, fakeProvider{packet: corereview.Packet{Verdict: corereview.VerdictPass}}, fakeClock{}, "task")
	if err != nil {
		t.Fatal(err)
	}
	if out.Verdict != corereview.VerdictFail || len(out.Findings) != 1 || out.Findings[0].ID != "workspace_mutation" {
		t.Fatalf("mutation guard output = %+v", out)
	}
}

func TestWorkspaceMutationsDetectsAddModifyAndDelete(t *testing.T) {
	t.Parallel()

	mutated := workspaceMutations([]string{"hash-a same", "hash-old modified", "hash-delete deleted"}, []string{"hash-a same", "hash-new modified", "hash-add added"})
	if len(mutated) != 4 {
		t.Fatalf("mutations = %+v", mutated)
	}
}
