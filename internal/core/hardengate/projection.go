package hardengate

import (
	"fmt"
	"strings"

	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// Kind is the stable harden gate projection state.
type Kind string

const (
	KindNotRun                Kind = "not_run"
	KindRoundOpen             Kind = "round_open"
	KindNeedsOperatorDecision Kind = "needs_operator_decision"
	KindPassed                Kind = "passed"
	KindStaleAfterDraft       Kind = "stale_after_draft"
	KindOverridden            Kind = "overridden"
	KindError                 Kind = "error"
	KindInvalid               Kind = "invalid"
)

// State is the single read model for draft hardening decisions.
type State struct {
	Kind                   Kind
	TaskID                 string
	Reason                 string
	Next                   string
	CurrentDigest          string
	LatestRound            spec.HardenRound
	HasRound               bool
	SameDraft              bool
	CanOpenRound           bool
	CanStartProvider       bool
	ProviderRerunBlocked   bool
	ApprovalReasonRequired bool
	Evidence               []string
	Blockers               []string
}

// Project reduces the current draft spec into harden gate state. The latest
// harden round is authoritative; top-level harden_status is treated as a
// projection that may be stale.
func Project(model spec.Model) State {
	digest := spec.HardenContractDigest(model)
	state := State{
		Kind:             KindNotRun,
		TaskID:           model.TaskID,
		Reason:           "hardening has not run",
		Next:             ApproveCommand(model.TaskID),
		CurrentDigest:    digest,
		CanOpenRound:     true,
		CanStartProvider: true,
	}
	round, ok := LatestRound(model)
	if !ok {
		return projectLegacyStatus(state, model.HardenStatus)
	}
	return projectRoundState(state, model, round)
}

func projectLegacyStatus(state State, status spec.HardenStatus) State {
	switch status {
	case "", spec.HardenNotRun:
		return state
	case spec.HardenInProgress:
		state.Kind = KindRoundOpen
		state.Reason = "draft hardening is in progress"
		state.Next = MarkPassedCommand(state.TaskID)
		state.CanOpenRound = false
		state.CanStartProvider = false
		state.ProviderRerunBlocked = true
		state.ApprovalReasonRequired = true
	case spec.HardenPassed:
		state.Kind = KindStaleAfterDraft
		state.Reason = "legacy harden pass is missing round evidence"
		state.Next = HardenCommand(state.TaskID)
		state.ApprovalReasonRequired = true
		state.Evidence = []string{"legacy harden_status: passed"}
		state.Blockers = []string{"legacy harden pass does not prove current draft"}
	case spec.HardenNeedsRevision:
		state.Kind = KindNeedsOperatorDecision
		state.Reason = "hardening found draft findings requiring operator judgment"
		state.Next = NeedsRevisionFollowUp(state.TaskID, true)
		state.ApprovalReasonRequired = true
		state.Evidence = []string{"legacy harden_status: needs_revision"}
		state.Blockers = []string{"draft needs revision from legacy harden_status"}
	case spec.HardenOverridden:
		state.Kind = KindOverridden
		state.Reason = "hardening was overridden by operator approval"
		state.Next = ApproveCommand(state.TaskID)
	case spec.HardenError:
		state.Kind = KindError
		state.Reason = "hardening provider or evidence failed"
		state.Next = HardenCommand(state.TaskID)
		state.ApprovalReasonRequired = true
		state.Evidence = []string{"legacy harden_status: error"}
		state.Blockers = []string{"hardening provider or evidence failed"}
	default:
		state.Kind = KindInvalid
		state.Reason = "unsupported harden status"
		state.Next = HardenCommand(state.TaskID)
		state.ApprovalReasonRequired = true
		state.Blockers = []string{"unsupported harden status: " + string(status)}
	}
	return state
}

func projectRoundState(state State, model spec.Model, round spec.HardenRound) State {
	digest := state.CurrentDigest
	state.LatestRound = round
	state.HasRound = true
	state.SameDraft = SameDraft(round, digest)
	status := roundStatus(model, round)
	state.Evidence = roundEvidence(round)
	state.Blockers = roundBlockers(round)
	switch status {
	case string(spec.HardenInProgress):
		state.Kind = KindRoundOpen
		state.Reason = "draft hardening is in progress"
		state.Next = MarkPassedCommand(model.TaskID)
		state.CanOpenRound = false
		state.CanStartProvider = false
		state.ProviderRerunBlocked = true
		state.ApprovalReasonRequired = true
	case string(spec.HardenPassed):
		if !state.SameDraft {
			state.Kind = KindStaleAfterDraft
			if strings.TrimSpace(round.SpecDigest) == "" {
				state.Reason = "latest harden pass is missing spec digest"
			} else {
				state.Reason = "latest harden pass is stale after draft edits"
			}
			state.Next = HardenCommand(model.TaskID)
			state.ApprovalReasonRequired = true
			state.Blockers = append(state.Blockers, "latest harden pass does not prove current draft")
			return state
		}
		state.Kind = KindPassed
		state.Reason = "hardening passed"
		state.Next = ApproveCommand(model.TaskID)
	case string(spec.HardenNeedsRevision):
		state.Kind = KindNeedsOperatorDecision
		state.Reason = "hardening found draft findings requiring operator judgment"
		state.Next = NeedsRevisionFollowUp(model.TaskID, true)
		state.ApprovalReasonRequired = true
		if state.SameDraft {
			state.CanOpenRound = false
			state.CanStartProvider = false
			state.ProviderRerunBlocked = true
		}
	case string(spec.HardenOverridden):
		state.Kind = KindOverridden
		state.Reason = fallback(round.Summary, "hardening was overridden by operator approval")
		state.Next = ApproveCommand(model.TaskID)
	case string(spec.HardenError):
		state.Kind = KindError
		state.Reason = fallback(round.Summary, "hardening provider or evidence failed")
		state.Next = HardenCommand(model.TaskID)
		state.ApprovalReasonRequired = true
	default:
		state.Kind = KindInvalid
		state.Reason = "latest harden round has unsupported status"
		state.Next = HardenCommand(model.TaskID)
		state.ApprovalReasonRequired = true
		state.Blockers = append(state.Blockers, "unsupported harden round status: "+status)
	}
	return state
}

// LatestRound returns the latest harden round when present.
func LatestRound(model spec.Model) (spec.HardenRound, bool) {
	if len(model.HardenRounds) == 0 {
		return spec.HardenRound{}, false
	}
	return model.HardenRounds[len(model.HardenRounds)-1], true
}

// SameDraft reports whether round was recorded against digest. Legacy rounds
// without a digest are never considered same-draft provider-blocking evidence.
func SameDraft(round spec.HardenRound, digest string) bool {
	return strings.TrimSpace(round.SpecDigest) != "" && strings.TrimSpace(round.SpecDigest) == strings.TrimSpace(digest)
}

// ApproveCommand formats the approval command.
func ApproveCommand(taskID string) string {
	return command("approve", taskID)
}

// HardenCommand formats the harden command.
func HardenCommand(taskID string) string {
	return command("harden", taskID)
}

// MarkPassedCommand formats the manual harden close command.
func MarkPassedCommand(taskID string) string {
	return HardenCommand(taskID) + " --mark-passed"
}

// NeedsRevisionFollowUp formats the operator decision prompt after findings.
func NeedsRevisionFollowUp(taskID string, provider bool) string {
	rerun := HardenCommand(taskID)
	if provider {
		rerun += " --provider <provider>"
	}
	return "operator decision: edit the draft for real shape blockers, then run " + rerun + "; or run " + ApproveCommand(taskID) + " --reason <reason> if the finding is rejected as bookkeeping/advisory/overengineering"
}

func command(name string, taskID string) string {
	if strings.TrimSpace(taskID) == "" {
		return "scafld " + name
	}
	return "scafld " + name + " " + taskID
}

func roundStatus(model spec.Model, round spec.HardenRound) string {
	if strings.TrimSpace(round.Status) != "" {
		return strings.TrimSpace(round.Status)
	}
	return strings.TrimSpace(string(model.HardenStatus))
}

func roundEvidence(round spec.HardenRound) []string {
	evidence := []string{"round: " + round.ID}
	if round.Summary != "" {
		evidence = append(evidence, "summary: "+round.Summary)
	}
	evidence = append(evidence, roundBlockers(round)...)
	return evidence
}

func roundBlockers(round spec.HardenRound) []string {
	var blockers []string
	if round.Shape.Decision != "" && round.Shape.Decision != coreharden.DecisionKeep {
		blockers = append(blockers, "shape decision requires revision: "+round.Shape.Decision)
	}
	if len(round.Shape.RequiredSpecEdits) > 0 {
		blockers = append(blockers, fmt.Sprintf("%d required spec edit(s)", len(round.Shape.RequiredSpecEdits)))
	}
	for _, observation := range round.Observations {
		if coreharden.ObservationBlocksApproval(coreharden.Observation{
			Dimension: observation.Dimension,
			Result:    observation.Result,
			Anchor:    observation.Anchor,
			Note:      observation.Note,
			Status:    observation.Status,
		}) {
			blockers = append(blockers, fmt.Sprintf("%s observation is still blocking", observation.Dimension))
		}
	}
	if len(blockers) == 0 && strings.TrimSpace(round.Status) == string(spec.HardenNeedsRevision) {
		return []string{"draft needs revision from " + round.ID}
	}
	return blockers
}

func fallback(value string, fallbackValue string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallbackValue
}
