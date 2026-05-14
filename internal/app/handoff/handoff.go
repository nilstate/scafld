package handoff

import (
	"context"
	"fmt"
	"strings"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec persistence port used by handoff rendering.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore loads session evidence for repair handoffs.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// Run renders the model-facing handoff for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (string, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return "", err
	}
	var ledger session.Session
	haveLedger := false
	if sessions != nil {
		if loaded, err := sessions.Load(ctx, model.TaskID); err == nil {
			ledger = loaded
			haveLedger = true
		}
	}
	next := handoffNext(model, ledger, haveLedger)
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff: %s\n\nStatus: %s\nNext: %s\n", model.Title, model.Status, next)
	writeTaskContract(&b, model)
	writeBuildPhase(&b, model)
	if haveLedger {
		writeAcceptanceEvidence(&b, model, ledger)
		writeBlockedAcceptance(&b, model, ledger)
		writeReviewGate(&b, model, ledger, next)
		writeCompletionAuthority(&b, model, ledger)
		writeLatestReviewFindings(&b, ledger)
	}
	return b.String(), nil
}

func handoffNext(model spec.Model, ledger session.Session, haveLedger bool) string {
	next := model.CurrentState.AllowedFollowUp
	if _, failed := latestReviewAttemptFailed(ledger); haveLedger && failed {
		return "scafld handoff " + model.TaskID
	}
	return next
}

func writeTaskContract(b *strings.Builder, model spec.Model) {
	if strings.TrimSpace(model.Summary) == "" && len(model.Scope) == 0 && len(model.Touchpoints) == 0 {
		return
	}
	b.WriteString("\n## Task Contract\n\n")
	if summary := strings.TrimSpace(model.Summary); summary != "" {
		fmt.Fprintf(b, "Summary: %s\n", summary)
	}
	writeStringList(b, "Scope", model.Scope, 5)
	writeStringList(b, "Touchpoints", model.Touchpoints, 8)
}

func writeBuildPhase(b *strings.Builder, model spec.Model) {
	if model.Status != spec.StatusActive && model.Status != spec.StatusBlocked {
		return
	}
	phaseID := strings.TrimSpace(model.CurrentState.CurrentPhase)
	if phaseID == "" || phaseID == "none" {
		return
	}
	b.WriteString("\n## Build Step\n\n")
	if phaseID == "final" {
		b.WriteString("Current step: final acceptance\n")
		fmt.Fprintf(b, "After repair or implementation, run `%s` to record evidence.\n", "scafld build "+model.TaskID)
		writeCriteriaList(b, "Final acceptance", model.Acceptance.Criteria)
		return
	}
	phase, ok := findPhase(model, phaseID)
	if !ok {
		fmt.Fprintf(b, "Current phase: %s\n", phaseID)
		fmt.Fprintf(b, "After implementation, run `%s` to record evidence.\n", "scafld build "+model.TaskID)
		return
	}
	fmt.Fprintf(b, "Current phase: %s (%s)\n", phase.ID, phase.Name)
	if strings.TrimSpace(phase.Objective) != "" {
		fmt.Fprintf(b, "Objective: %s\n", phase.Objective)
	}
	writeStringList(b, "Changes", phase.Changes, 12)
	writeCriteriaList(b, "Phase acceptance", phase.Acceptance)
	fmt.Fprintf(b, "After implementing this phase, run `%s` to record evidence.\n", "scafld build "+model.TaskID)
}

func findPhase(model spec.Model, phaseID string) (spec.Phase, bool) {
	for _, phase := range model.Phases {
		if phase.ID == phaseID {
			return phase, true
		}
	}
	return spec.Phase{}, false
}

func writeCriteriaList(b *strings.Builder, title string, criteria []spec.Criterion) {
	if len(criteria) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", title)
	for _, criterion := range criteria {
		fmt.Fprintf(b, "- %s", criterionTitle(criterion))
		if criterion.ID != "" {
			fmt.Fprintf(b, " (`%s`)", criterion.ID)
		}
		if strings.TrimSpace(criterion.Command) != "" {
			fmt.Fprintf(b, "\n  Command: `%s`", criterion.Command)
		}
		if criterion.ExpectedKind != "" {
			fmt.Fprintf(b, "\n  Expected kind: `%s`", criterion.ExpectedKind)
		}
		b.WriteString("\n")
	}
}

func writeStringList(b *strings.Builder, title string, values []string, limit int) {
	var kept []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			kept = append(kept, value)
		}
	}
	if len(kept) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", title)
	for idx, value := range kept {
		if idx >= limit {
			fmt.Fprintf(b, "- ... %d more\n", len(kept)-limit)
			return
		}
		fmt.Fprintf(b, "- %s\n", value)
	}
}

func writeAcceptanceEvidence(b *strings.Builder, model spec.Model, ledger session.Session) {
	criteria := model.AllCriteria()
	if len(criteria) == 0 {
		return
	}
	replayed := session.Replay(ledger)
	b.WriteString("\n## Acceptance Evidence\n\n")
	for _, criterion := range criteria {
		state, ok := replayed.CriterionStates[criterion.ID]
		if !ok {
			fmt.Fprintf(b, "- [pending] %s", criterionTitle(criterion))
			if criterion.ID != "" {
				fmt.Fprintf(b, " (`%s`)", criterion.ID)
			}
			b.WriteString(": no evidence recorded\n")
			if strings.TrimSpace(criterion.Command) != "" {
				fmt.Fprintf(b, "  Command: `%s`\n", criterion.Command)
			}
			continue
		}
		reason := strings.TrimSpace(state.Reason)
		if reason == "" {
			reason = state.Status
		}
		fmt.Fprintf(b, "- [%s] %s", state.Status, criterionTitle(criterion))
		if criterion.ID != "" {
			fmt.Fprintf(b, " (`%s`)", criterion.ID)
		}
		fmt.Fprintf(b, ": %s\n", reason)
		if state.SourceID != "" {
			fmt.Fprintf(b, "  Source event: `%s`\n", state.SourceID)
		}
		if strings.TrimSpace(criterion.Command) != "" {
			fmt.Fprintf(b, "  Command: `%s`\n", criterion.Command)
		}
	}
}

func writeBlockedAcceptance(b *strings.Builder, model spec.Model, ledger session.Session) {
	if model.Status != spec.StatusBlocked {
		return
	}
	replayed := session.Replay(ledger)
	var rows []string
	for _, criterion := range blockedCriteria(model) {
		state, ok := replayed.CriterionStates[criterion.ID]
		source, _ := entryBySource(ledger, state.SourceID)
		context := phaseDependencyContext(model, criterion.PhaseID)
		switch {
		case !ok:
			rows = append(rows, criterionHandoffRow(criterion, "pending", "no evidence recorded", "", context))
		case state.Status != "pass":
			reason := state.Reason
			if reason == "" {
				reason = "acceptance did not pass"
			}
			rows = append(rows, criterionHandoffRow(criterion, state.Status, reason, source.Path, context))
		}
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("\n## Blocked Acceptance\n\n")
	for _, row := range rows {
		b.WriteString(row)
	}
}

func blockedCriteria(model spec.Model) []spec.Criterion {
	switch strings.TrimSpace(model.CurrentState.CurrentPhase) {
	case "", "none":
		return model.AllCriteria()
	case "final":
		return append([]spec.Criterion(nil), model.Acceptance.Criteria...)
	default:
		for _, phase := range model.Phases {
			if phase.ID == model.CurrentState.CurrentPhase {
				criteria := append([]spec.Criterion(nil), phase.Acceptance...)
				for i := range criteria {
					if criteria[i].PhaseID == "" {
						criteria[i].PhaseID = phase.ID
					}
				}
				return criteria
			}
		}
		return model.AllCriteria()
	}
}

func criterionHandoffRow(criterion spec.Criterion, status string, reason string, diagnosticPath string, phaseContext string) string {
	var b strings.Builder
	title := criterionTitle(criterion)
	fmt.Fprintf(&b, "- [%s] %s", status, title)
	if criterion.ID != "" && title != criterion.ID {
		fmt.Fprintf(&b, " (`%s`)", criterion.ID)
	}
	if reason != "" {
		fmt.Fprintf(&b, ": %s", reason)
	}
	b.WriteString("\n")
	if strings.TrimSpace(criterion.Command) != "" {
		fmt.Fprintf(&b, "  Command: `%s`\n", criterion.Command)
	}
	if criterion.ExpectedKind != "" {
		fmt.Fprintf(&b, "  Expected kind: `%s`\n", criterion.ExpectedKind)
	}
	if phaseContext != "" {
		fmt.Fprintf(&b, "  Phase context: %s\n", phaseContext)
	}
	if diagnosticPath != "" {
		fmt.Fprintf(&b, "  Evidence: `%s`\n", diagnosticPath)
	}
	return b.String()
}

func entryBySource(ledger session.Session, sourceID string) (session.Entry, bool) {
	if sourceID == "" {
		return session.Entry{}, false
	}
	for _, entry := range ledger.Entries {
		if entry.ID == sourceID {
			return entry, true
		}
	}
	return session.Entry{}, false
}

func phaseDependencyContext(model spec.Model, phaseID string) string {
	if phaseID == "" {
		return "global acceptance"
	}
	for _, phase := range model.Phases {
		if phase.ID != phaseID {
			continue
		}
		if len(phase.Dependencies) == 0 {
			return "phase " + phase.ID + " has no declared dependencies"
		}
		return "phase " + phase.ID + " depends on " + strings.Join(phase.Dependencies, ", ")
	}
	return "phase " + phaseID
}

func criterionTitle(criterion spec.Criterion) string {
	title := strings.TrimSpace(criterion.Title)
	if title == "" {
		title = criterion.ID
	}
	return title
}

func writeReviewGate(b *strings.Builder, model spec.Model, ledger session.Session, next string) {
	if model.Status != spec.StatusReview {
		return
	}
	b.WriteString("\n## Review Gate\n\n")
	if model.CurrentState.ReviewGate != "" {
		fmt.Fprintf(b, "Gate: %s\n", model.CurrentState.ReviewGate)
	}
	if model.CurrentState.Reason != "" {
		fmt.Fprintf(b, "Reason: %s\n", model.CurrentState.Reason)
	}
	if next != "" {
		fmt.Fprintf(b, "Allowed command: `%s`\n", next)
	}
	if attempt, ok := latestReviewAttemptFailed(ledger); ok {
		fmt.Fprintf(b, "Latest review attempt: failed\n")
		if attempt.Reason != "" {
			fmt.Fprintf(b, "Attempt reason: %s\n", attempt.Reason)
		}
		if attempt.Path != "" {
			fmt.Fprintf(b, "Attempt diagnostic: `%s`\n", attempt.Path)
		}
	}
	if baseline, ok := session.FirstWorkspaceBaseline(ledger); ok {
		fmt.Fprintf(b, "Workspace baseline: `%s`\n", baseline.ID)
	}
	b.WriteString("\nReviewer focus:\n")
	b.WriteString("- Attack the diff against the approved contract and recorded baseline.\n")
	b.WriteString("- Treat session evidence as trusted state; treat this handoff as transport.\n")
	b.WriteString("- Return a structured review verdict through `scafld review`, not by editing the spec.\n")
}

func writeCompletionAuthority(b *strings.Builder, model spec.Model, ledger session.Session) {
	if model.Status != spec.StatusCompleted {
		return
	}
	authority := corecompletion.TerminalAuthority(ledger)
	b.WriteString("\n## Completion Authority\n\n")
	fmt.Fprintf(b, "Status: %s\n", authority.Status())
	fmt.Fprintf(b, "Kind: %s\n", authority.Kind())
	if authority.Provider() != "" || authority.Verdict() != "" {
		fmt.Fprintf(b, "Review: %s", authority.Verdict())
		if authority.Provider() != "" {
			fmt.Fprintf(b, " by %s", authority.Provider())
		}
		b.WriteString("\n")
	}
	if summary := authority.Summary(); summary != "" {
		fmt.Fprintf(b, "Summary: %s\n", summary)
	}
	if authority.ReviewEntry.ID != "" {
		fmt.Fprintf(b, "Review event: `%s`\n", authority.ReviewEntry.ID)
	}
	if authority.CompleteEntry.ID != "" {
		fmt.Fprintf(b, "Complete event: `%s`\n", authority.CompleteEntry.ID)
	}
	if !authority.Valid {
		if authority.Reason != "" {
			fmt.Fprintf(b, "Integrity error: %s\n", authority.Reason)
		}
		if authority.Actual != "" {
			fmt.Fprintf(b, "Actual: %s\n", authority.Actual)
		}
	}
	b.WriteString("Archived tasks are immutable; create a new task for follow-up work.\n")
}

func latestReviewAttemptFailed(ledger session.Session) (session.Entry, bool) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		switch ledger.Entries[i].Type {
		case "review_attempt":
			return ledger.Entries[i], ledger.Entries[i].Status == "failed"
		case "build", "criterion", "phase", "review", "approval", session.EntryWorkspaceBaseline, "fail", "cancel":
			return session.Entry{}, false
		}
	}
	return session.Entry{}, false
}

func writeLatestReviewFindings(b *strings.Builder, ledger session.Session) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
		case "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel":
			return
		default:
			continue
		}
		dossier, ok := corereview.DecodeDossier(entry.Output)
		if !ok || len(dossier.Findings) == 0 {
			return
		}
		fmt.Fprintf(b, "\n## Review Dossier\n\nVerdict: %s\nMode: %s\n", entry.Status, dossier.Mode)
		if dossier.Summary != "" {
			fmt.Fprintf(b, "Summary: %s\n", dossier.Summary)
		}
		fmt.Fprintf(b, "\nFindings:\n")
		for _, finding := range dossier.Findings {
			blocking := "non-blocking"
			if corereview.BlocksCompletion(finding) {
				blocking = "blocks completion"
			}
			fmt.Fprintf(b, "- [%s/%s] %s: %s\n", finding.Severity, blocking, finding.ID, finding.Summary)
			if finding.Location != nil && finding.Location.Path != "" {
				if finding.Location.Line > 0 {
					fmt.Fprintf(b, "  - Location: `%s:%d`\n", finding.Location.Path, finding.Location.Line)
				} else {
					fmt.Fprintf(b, "  - Location: `%s`\n", finding.Location.Path)
				}
			}
			if finding.Evidence != "" {
				fmt.Fprintf(b, "  - Evidence: %s\n", finding.Evidence)
			}
			if finding.Validation != "" {
				fmt.Fprintf(b, "  - Validate: %s\n", finding.Validation)
			}
		}
		return
	}
}
