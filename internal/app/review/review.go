package review

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec persistence port used by review.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by review.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Provider is the review provider port.
type Provider interface {
	Invoke(context.Context, review.Request) (review.Packet, error)
}

// WorkspaceStatus is the mutation-guard workspace state port.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies review timestamps.
type Clock interface{ Now() time.Time }

// Output describes a completed review run.
type Output struct {
	TaskID   string           `json:"task_id"`
	Verdict  string           `json:"verdict"`
	Findings []review.Finding `json:"findings"`
	Next     string           `json:"next"`
}

// Input describes the task and review agenda to run.
type Input struct {
	TaskID string
	Passes []Pass
}

// Pass describes one configured review pass included in the provider prompt.
type Pass struct {
	ID          string
	Category    string
	Order       int
	Title       string
	Description string
}

// Run executes the review gate for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, provider Provider, clock Clock, taskID string) (Output, error) {
	return RunWithInput(ctx, specs, sessions, workspace, provider, clock, Input{TaskID: taskID})
}

// RunWithInput executes the review gate using an explicit review agenda.
func RunWithInput(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, provider Provider, clock Clock, input Input) (Output, error) {
	model, path, err := specs.Load(ctx, input.TaskID)
	if err != nil {
		return Output{}, err
	}
	before, err := workspaceSnapshot(ctx, workspace)
	if err != nil {
		return Output{}, err
	}
	packet, err := provider.Invoke(ctx, review.Request{TaskID: model.TaskID, Prompt: promptForModel(model, input.Passes)})
	after, mutationErr := workspaceSnapshot(ctx, workspace)
	if mutationErr != nil {
		return Output{}, mutationErr
	}
	if mutated := workspaceMutations(before, after); len(mutated) > 0 {
		packet = review.Packet{
			Verdict: review.VerdictFail,
			Findings: []review.Finding{{
				ID:       "workspace_mutation",
				Severity: review.SeverityBlocking,
				Summary:  "provider mutated workspace during review: " + strings.Join(mutated, ", "),
			}},
		}
		err = nil
	}
	if err != nil {
		return Output{}, err
	}
	if err := review.ValidatePacket(packet); err != nil {
		return Output{}, err
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = packet.Verdict
	model.Review.Findings = packet.Findings
	model.CurrentState.ReviewGate = packet.Verdict
	next, command := nextForVerdict(model.TaskID, packet.Verdict)
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	ledger, err := sessions.Append(ctx, model.TaskID, session.Entry{
		Type:     "review",
		Status:   packet.Verdict,
		Reason:   reviewReason(packet),
		Provider: packet.Provider,
		Output:   review.EncodeFindings(packet.Findings),
	}, now)
	if err != nil {
		return Output{}, err
	}
	if loaded, loadErr := sessions.Load(ctx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.Status = spec.StatusReview
	model.Review.Status = "completed"
	model.Review.Verdict = packet.Verdict
	model.Review.Findings = packet.Findings
	model.CurrentState.ReviewGate = packet.Verdict
	model.CurrentState.Next = next
	model.CurrentState.AllowedFollowUp = command
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, err
	}
	return Output{TaskID: model.TaskID, Verdict: packet.Verdict, Findings: packet.Findings, Next: command}, nil
}

func reviewReason(packet review.Packet) string {
	blocking := review.CountBlocking(packet.Findings)
	if len(packet.Findings) == 0 {
		return "review gate " + packet.Verdict
	}
	return fmt.Sprintf("review gate %s: %d finding(s), %d blocking", packet.Verdict, len(packet.Findings), blocking)
}

func workspaceSnapshot(ctx context.Context, workspace WorkspaceStatus) ([]string, error) {
	if workspace == nil {
		return nil, nil
	}
	files, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), files...), nil
}

func workspaceMutations(before []string, after []string) []string {
	seen := map[string]bool{}
	for _, path := range before {
		seen[path] = true
	}
	var mutated []string
	for _, path := range after {
		if !seen[path] {
			mutated = append(mutated, path)
		}
		delete(seen, path)
	}
	for path := range seen {
		mutated = append(mutated, path)
	}
	return mutated
}

func nextForVerdict(taskID string, verdict string) (string, string) {
	if verdict == "pass" {
		return "complete", "scafld complete " + taskID
	}
	return "repair", "scafld handoff " + taskID
}

func promptForModel(model spec.Model, passes []Pass) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review %s\n\n", model.TaskID)
	fmt.Fprintf(&b, "Title: %s\nStatus: %s\n\n", model.Title, model.Status)
	if strings.TrimSpace(model.Summary) != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", strings.TrimSpace(model.Summary))
	}
	if len(model.Objectives) > 0 {
		b.WriteString("## Objectives\n\n")
		for _, objective := range model.Objectives {
			fmt.Fprintf(&b, "- %s\n", objective)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Acceptance Criteria\n\n")
	for _, criterion := range model.AllCriteria() {
		fmt.Fprintf(&b, "- %s (%s): %s\n", criterion.ID, criterion.ExpectedKind, criterion.Command)
		if strings.TrimSpace(criterion.Status) != "" {
			fmt.Fprintf(&b, "  - Status: %s\n", criterion.Status)
		}
		if strings.TrimSpace(criterion.Evidence) != "" {
			fmt.Fprintf(&b, "  - Evidence: %s\n", criterion.Evidence)
		}
	}
	writeReviewPasses(&b, passes)
	b.WriteString("\nReview mode is read-only. Do not run build, test, or mutation commands; treat recorded acceptance evidence above as already executed. Return a ReviewPacket JSON object with `verdict` and `findings`. If your transport only supports line frames, emit `finding` frames with severity `blocking` or `non_blocking`, then a `verdict` frame with verdict `pass` or `fail`.\n")
	return b.String()
}

func writeReviewPasses(b *strings.Builder, passes []Pass) {
	if len(passes) == 0 {
		return
	}
	sorted := append([]Pass(nil), passes...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Order == sorted[j].Order {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Order < sorted[j].Order
	})
	b.WriteString("\n## Review Focus\n\n")
	for _, pass := range sorted {
		title := strings.TrimSpace(pass.Title)
		if title == "" {
			title = pass.ID
		}
		category := strings.TrimSpace(pass.Category)
		if category == "" {
			category = "review"
		}
		fmt.Fprintf(b, "- %s: %s", category, title)
		if description := strings.TrimSpace(pass.Description); description != "" {
			fmt.Fprintf(b, " - %s", description)
		}
		b.WriteString("\n")
	}
}
