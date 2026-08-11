package reconcile

import (
	"strings"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// PhaseBlockFields documents the session-backed fields projected into phase state.
var PhaseBlockFields = map[string]string{
	"status":     "projected phase state",
	"reason":     "human-readable source reason",
	"updated_at": "source event timestamp",
	"source_id":  "session entry identifier",
}

// Projection is the session-derived view that can be replayed into a spec.
type Projection struct {
	TaskID string
	Lines  []string
}

// Idempotent returns a normalized projection without changing semantic state.
func Idempotent(current Projection) Projection {
	return Projection{
		TaskID: current.TaskID,
		Lines:  append([]string(nil), current.Lines...),
	}
}

// FromSession projects replayed session state into a spec model.
func FromSession(model spec.Model, ledger session.Session) spec.Model {
	replayed := session.Replay(ledger)
	next := model
	for i, criterion := range next.Acceptance.Criteria {
		projectCriterionState(&next.Acceptance.Criteria[i], criterion, replayed.CriterionStates[criterion.ID])
	}
	for pi, phase := range next.Phases {
		if state, ok := replayed.PhaseBlocks[phase.ID]; ok {
			next.Phases[pi].Status = state.Status
			next.Phases[pi].Reason = state.Reason
		}
		for ci, criterion := range phase.Acceptance {
			withPhase := criterion
			if withPhase.PhaseID == "" {
				withPhase.PhaseID = phase.ID
			}
			projectCriterionState(&next.Phases[pi].Acceptance[ci], withPhase, replayed.CriterionStates[criterion.ID])
		}
		if next.Phases[pi].Status == "completed" && !phaseAcceptancePassed(next.Phases[pi]) {
			next.Phases[pi].Status = "active"
			next.Phases[pi].Reason = acceptanceStaleReason
		}
	}
	if len(replayed.Entries) > 0 {
		last := replayed.Entries[len(replayed.Entries)-1]
		next.CurrentState.LatestRunnerUpdate = last.RecordedAt
		next.CurrentState.Reason = last.Reason
	}
	projectLifecycle(&next, replayed)
	for i := len(replayed.Entries) - 1; i >= 0; i-- {
		entry := replayed.Entries[i]
		if entry.Type != "review" {
			continue
		}
		next.Review.Status = "completed"
		next.Review.Verdict = entry.Status
		if dossier, ok := corereview.DecodeDossier(entry.Output); ok {
			next.Review.Mode = dossier.Mode
			next.Review.Summary = dossier.Summary
			next.Review.Findings = dossier.Findings
			next.Review.AttackLog = dossier.AttackLog
			next.Review.Budget = dossier.Budget
			next.Review.Provider = dossier.Provider
			next.Review.Model = dossier.Model
			next.Review.OutputFormat = dossier.OutputFormat
			next.Review.Normalizations = dossier.Normalizations
		}
		break
	}
	projectAcceptanceFreshness(&next)
	return next
}

const acceptanceStaleReason = "acceptance evidence stale after criterion contract change"

func projectCriterionState(target *spec.Criterion, criterion spec.Criterion, state session.StateRecord) {
	target.Status = "pending"
	target.Evidence = ""
	target.SourceEvent = ""
	if !criterionEvidenceMatches(criterion, state) {
		return
	}
	target.Status = state.Status
	target.Evidence = state.Reason
	target.SourceEvent = state.SourceID
}

func criterionEvidenceMatches(criterion spec.Criterion, state session.StateRecord) bool {
	if state.SourceID == "" {
		return false
	}
	if strings.TrimSpace(state.Command) != strings.TrimSpace(criterion.Command) {
		return false
	}
	if strings.TrimSpace(state.ExpectedKind) == "" || state.ExpectedKind != string(criterion.ExpectedKind) {
		return false
	}
	if strings.TrimSpace(state.CriterionType) == "" || normalizeCriterionType(state.CriterionType) != normalizeCriterionType(criterion.Type) {
		return false
	}
	if criterion.PhaseID != "" {
		return state.PhaseID == criterion.PhaseID
	}
	if state.PhaseID != "" {
		return false
	}
	return true
}

func normalizeCriterionType(value string) string {
	switch value {
	case "":
		return "command"
	default:
		return value
	}
}

func phaseAcceptancePassed(phase spec.Phase) bool {
	for _, criterion := range phase.Acceptance {
		if criterion.Status != "pass" {
			return false
		}
	}
	return true
}

func projectAcceptanceFreshness(model *spec.Model) {
	if model.Status == spec.StatusCompleted || model.Status == spec.StatusFailed || model.Status == spec.StatusCancelled {
		return
	}
	if len(model.Acceptance.Criteria) > 0 {
		for _, criterion := range model.Acceptance.Criteria {
			if criterion.Status != "pass" {
				applyStaleAcceptanceState(model, "final")
				return
			}
		}
	}
	for _, phase := range model.Phases {
		if len(phase.Acceptance) == 0 || phase.Status == "completed" {
			continue
		}
		applyStaleAcceptanceState(model, phase.ID)
		return
	}
}

func applyStaleAcceptanceState(model *spec.Model, phaseID string) {
	if model.Status != spec.StatusReview {
		return
	}
	model.Status = spec.StatusActive
	model.CurrentState.CurrentPhase = phaseID
	model.CurrentState.Next = "build"
	model.CurrentState.Reason = acceptanceStaleReason
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func projectLifecycle(model *spec.Model, ledger session.Session) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "approval":
			model.Status = spec.StatusApproved
			projectApprovedState(model, entry.Reason)
			return
		case "build":
			model.Status = spec.Status(entry.Status)
			projectBuildState(model, entry.Reason)
			return
		case "review":
			model.Status = spec.StatusReview
			projectReviewState(model, entry.Reason)
			return
		case "complete":
			model.Status = spec.StatusCompleted
			projectTerminalState(model, entry.Reason)
			return
		case "fail":
			model.Status = spec.StatusFailed
			projectTerminalState(model, entry.Reason)
			return
		case "cancel":
			model.Status = spec.StatusCancelled
			projectTerminalState(model, entry.Reason)
			return
		}
	}
}

func projectApprovedState(model *spec.Model, reason string) {
	model.CurrentState.CurrentPhase = "none"
	model.CurrentState.Next = "build"
	model.CurrentState.Reason = fallbackReason(reason, model.CurrentState.Reason)
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld build " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func projectBuildState(model *spec.Model, reason string) {
	switch model.Status {
	case spec.StatusActive:
		phaseID := currentBuildPhase(*model)
		model.CurrentState.CurrentPhase = phaseID
		model.CurrentState.Next = "build"
		model.CurrentState.Reason = fallbackReason(reason, "build active")
		model.CurrentState.Blockers = "none"
		model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
		model.CurrentState.ReviewGate = "not_started"
	case spec.StatusBlocked:
		phaseID := currentBlockedPhase(*model)
		model.CurrentState.CurrentPhase = phaseID
		model.CurrentState.Next = "repair"
		model.CurrentState.Reason = fallbackReason(reason, "build blocked")
		model.CurrentState.Blockers = model.CurrentState.Reason
		model.CurrentState.AllowedFollowUp = "scafld handoff " + model.TaskID
		model.CurrentState.ReviewGate = "not_started"
	case spec.StatusReview:
		projectReviewState(model, reason)
	}
}

func projectReviewState(model *spec.Model, reason string) {
	model.CurrentState.CurrentPhase = "final"
	model.CurrentState.Next = "review"
	model.CurrentState.Reason = fallbackReason(reason, "build completed; ready for review")
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "scafld review " + model.TaskID
	model.CurrentState.ReviewGate = "not_started"
}

func projectTerminalState(model *spec.Model, reason string) {
	model.CurrentState.CurrentPhase = "none"
	model.CurrentState.Next = "none"
	model.CurrentState.Reason = fallbackReason(reason, model.CurrentState.Reason)
	model.CurrentState.Blockers = "none"
	model.CurrentState.AllowedFollowUp = "none"
}

func currentBuildPhase(model spec.Model) string {
	for _, phase := range model.Phases {
		if phase.Status == "active" {
			return phase.ID
		}
	}
	if phaseID, ok := firstIncompletePhase(model); ok {
		return phaseID
	}
	return "final"
}

func currentBlockedPhase(model spec.Model) string {
	for _, phase := range model.Phases {
		if phase.Status == "blocked" {
			return phase.ID
		}
	}
	return currentBuildPhase(model)
}

func firstIncompletePhase(model spec.Model) (string, bool) {
	for _, phase := range model.Phases {
		if phase.Status != "completed" {
			return phase.ID, true
		}
	}
	return "", false
}

func fallbackReason(reason string, fallback string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	return strings.TrimSpace(fallback)
}
