package handoff

import (
	"context"
	"fmt"
	"strings"
	"time"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
	"github.com/nilstate/scafld/v2/internal/core/reviewmaterial"
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

// WorkspaceStatus captures current workspace state for handoff projection.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

type workspaceMaterialStatus interface {
	MaterialSeal(context.Context, []string) (reviewevidence.MaterialSeal, error)
}

// Run renders the model-facing handoff for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string, workspaces ...WorkspaceStatus) (string, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return "", err
	}
	var ledger session.Session
	var reviewState reviewgate.State
	haveLedger := false
	if sessions != nil {
		if loaded, err := sessions.Load(ctx, model.TaskID); err == nil {
			ledger = loaded
			reviewState = reviewgate.Project(ledger, model, reviewgate.Options{Now: time.Now().UTC()})
			haveLedger = true
		}
	}
	next := handoffNext(model, reviewState, haveLedger)
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff: %s\n\nStatus: %s\nNext: %s\n", model.Title, model.Status, next)
	if haveLedger {
		writeTaskMaterial(ctx, &b, model, ledger, firstWorkspace(workspaces), reviewState.Authority)
	}
	writeTaskContract(&b, model)
	writeBuildPhase(&b, model)
	if haveLedger {
		writeAcceptanceEvidence(&b, model, ledger)
		writeBlockedAcceptance(&b, model, ledger)
		writeReviewGate(&b, model, ledger, reviewState, next)
		writeCompletionAuthority(&b, model, ledger)
		writeLatestReviewFindings(&b, reviewState)
	}
	return b.String(), nil
}

func firstWorkspace(workspaces []WorkspaceStatus) WorkspaceStatus {
	if len(workspaces) == 0 {
		return nil
	}
	return workspaces[0]
}

func handoffNext(model spec.Model, state reviewgate.State, haveLedger bool) string {
	next := model.CurrentState.AllowedFollowUp
	if haveLedger && model.Status == spec.StatusReview && state.Next != "" {
		return state.Next
	}
	return next
}

func writeTaskMaterial(ctx context.Context, b *strings.Builder, model spec.Model, ledger session.Session, workspace WorkspaceStatus, authority reviewgate.Authority) {
	var current []string
	hasCurrent := false
	if workspace != nil {
		if snapshot, err := workspace.ChangedFiles(ctx); err == nil {
			current = snapshot
			hasCurrent = true
		}
	}
	currentMaterialDigest := ""
	hasCurrentMaterialDigest := false
	if workspace != nil && authority.Valid && reviewedScopePresent(authority.ReviewEntry.ReviewedScope) {
		if material, ok := workspace.(workspaceMaterialStatus); ok {
			if seal, err := material.MaterialSeal(ctx, authority.ReviewEntry.ReviewedScope); err == nil {
				currentMaterialDigest = seal.Digest
				hasCurrentMaterialDigest = true
			}
		}
	}
	projection := reviewmaterial.Project(reviewmaterial.Input{
		Model:                    model,
		Ledger:                   ledger,
		CurrentSnapshot:          current,
		HasCurrentSnapshot:       hasCurrent,
		Authority:                authority,
		CurrentMaterialDigest:    currentMaterialDigest,
		HasCurrentMaterialDigest: hasCurrentMaterialDigest,
	})
	if projection.Empty() {
		return
	}

	b.WriteString("\n## Task Material\n\n")
	if projection.MaterialStatus != "" {
		fmt.Fprintf(b, "Material status: %s\n", projection.MaterialStatus)
	}
	writeStringList(b, "Scope", projection.Scope, 12)
	writeStringList(b, "Baseline paths", projection.BaselinePaths, 12)
	writeStringList(b, "Task changes since baseline", projection.TaskChanges, 12)
	writeStringList(b, "Ambient drift outside task scope", projection.AmbientDrift, 8)
	if len(projection.ReviewedScope) > 0 {
		writeStringList(b, "Reviewed scope", projection.ReviewedScope, 12)
	}
	b.WriteString("\nSearch discipline:\n")
	b.WriteString("- Use `rg --hidden` for repository checks that must include untracked files; avoid `git grep` for public-surface or legacy-shape gates.\n")
	b.WriteString("- Keep `.git/**`, `.scafld/runs/**`, `.scafld/receipts/**`, and generated release output out of manual searches unless they are the target.\n")
}

func reviewedScopePresent(scope []string) bool {
	for _, item := range scope {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
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

func writeReviewGate(b *strings.Builder, model spec.Model, ledger session.Session, state reviewgate.State, next string) {
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
	if state.HasAttempt {
		status := state.LatestAttempt.Status
		if state.LatestAttempt.Stale {
			status = "stale"
		}
		fmt.Fprintf(b, "Latest review attempt: %s\n", status)
		if state.LatestAttempt.Reason != "" {
			fmt.Fprintf(b, "Attempt reason: %s\n", state.LatestAttempt.Reason)
		}
		if state.LatestAttempt.HasLease {
			fmt.Fprintf(b, "Attempt lease expires: `%s`\n", state.LatestAttempt.LeaseExpiresAt.Format(time.RFC3339Nano))
		}
		if state.LatestAttempt.DiagnosticPath != "" {
			fmt.Fprintf(b, "Attempt diagnostic: `%s`\n", state.LatestAttempt.DiagnosticPath)
		}
	}
	if baseline, ok := session.FirstWorkspaceBaseline(ledger); ok {
		fmt.Fprintf(b, "Workspace baseline: `%s`\n", baseline.ID)
	}
	switch state.Kind {
	case reviewgate.KindAttemptRunning:
		b.WriteString("\nReview in progress:\n")
		b.WriteString("- Do not start another review while the current attempt lease is active.\n")
		b.WriteString("- Check status again after the provider returns.\n")
		return
	case reviewgate.KindAttemptStale:
		b.WriteString("\nStale attempt recovery:\n")
		fmt.Fprintf(b, "- Run `%s` to abandon the stale attempt and start a new leased attempt.\n", "scafld review "+model.TaskID)
		b.WriteString("- Do not complete from older passing review evidence after a later attempt.\n")
		return
	case reviewgate.KindAttemptFailed:
		b.WriteString("\nProvider repair focus:\n")
		b.WriteString("- Fix provider availability, permissions, timeout, or dossier output shape.\n")
		fmt.Fprintf(b, "- Then run `%s` for a new accepted review attempt.\n", "scafld review "+model.TaskID)
		b.WriteString("- Do not complete from an older passing review after a later failed attempt.\n")
		return
	case reviewgate.KindReviewFailed:
		b.WriteString("\nRepair focus:\n")
		b.WriteString("- Repair the completion-blocking findings in the Review Dossier below.\n")
		fmt.Fprintf(b, "- After repair, run `%s` to refresh acceptance evidence.\n", "scafld build "+model.TaskID)
		fmt.Fprintf(b, "- Then run `%s` for a new adversarial verdict.\n", "scafld review "+model.TaskID)
		b.WriteString("- Do not run `scafld complete` until a later review passes after refreshed evidence.\n")
		return
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

func writeLatestReviewFindings(b *strings.Builder, state reviewgate.State) {
	if !state.HasReview || !state.HasDossier || len(state.Dossier.Findings) == 0 {
		return
	}
	entry := state.LatestReview
	dossier := state.Dossier
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
}
