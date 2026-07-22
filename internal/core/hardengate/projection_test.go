package hardengate

import (
	"testing"

	coreharden "github.com/nilstate/scafld/v2/internal/core/harden"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestProjectBlocksSameDraftProviderRerunAfterNeedsRevision(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	digest := spec.HardenContractDigest(model)
	model.HardenStatus = spec.HardenNeedsRevision
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenNeedsRevision),
		SpecDigest: digest,
		Observations: []spec.HardenObservation{{
			Dimension: "design",
			Result:    coreharden.ResultBlocks,
			Anchor:    "spec_gap:Summary",
			Status:    coreharden.StatusOpen,
		}},
	}}
	state := Project(model)
	if state.Kind != KindNeedsOperatorDecision || !state.ProviderRerunBlocked || !state.ApprovalReasonRequired {
		t.Fatalf("state = %+v", state)
	}
	if state.CanStartProvider {
		t.Fatalf("same-draft needs_revision must block provider rerun: %+v", state)
	}
}

func TestProjectAllowsProviderAfterDraftRevision(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenNeedsRevision
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenNeedsRevision),
		SpecDigest: spec.HardenContractDigest(model),
	}}
	model.Summary = "revised summary"
	state := Project(model)
	if state.Kind != KindNeedsOperatorDecision || state.ProviderRerunBlocked || !state.CanStartProvider {
		t.Fatalf("state = %+v", state)
	}
}

func TestProjectUsesLatestRoundOverStaleProjection(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenInProgress
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenPassed),
		SpecDigest: spec.HardenContractDigest(model),
	}}
	state := Project(model)
	if state.Kind != KindPassed || state.ApprovalReasonRequired {
		t.Fatalf("state = %+v", state)
	}
}

func TestProjectMarksPassedRoundStaleAfterDraftEdit(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenPassed
	model.HardenRounds = []spec.HardenRound{{
		ID:         "round-1",
		Status:     string(spec.HardenPassed),
		SpecDigest: spec.HardenContractDigest(model),
	}}
	model.Summary = "changed"
	state := Project(model)
	if state.Kind != KindStaleAfterDraft || !state.CanStartProvider || !state.ApprovalReasonRequired {
		t.Fatalf("state = %+v", state)
	}
}

func TestProjectTreatsDigestlessPassedRoundAsStale(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenPassed
	model.HardenRounds = []spec.HardenRound{{
		ID:     "round-1",
		Status: string(spec.HardenPassed),
	}}
	state := Project(model)
	if state.Kind != KindStaleAfterDraft || !state.ApprovalReasonRequired {
		t.Fatalf("digestless pass should require fresh harden or reasoned override: %+v", state)
	}
	if state.Reason != "latest harden pass is missing spec digest" {
		t.Fatalf("reason = %q", state.Reason)
	}
}

func TestProjectRequiresReasonForOpenOrErrorHarden(t *testing.T) {
	t.Parallel()

	for _, status := range []spec.HardenStatus{spec.HardenInProgress, spec.HardenError} {
		model := fixtureModel()
		model.HardenStatus = status
		model.HardenRounds = []spec.HardenRound{{ID: "round-1", Status: string(status)}}
		state := Project(model)
		if !state.ApprovalReasonRequired {
			t.Fatalf("%s state should require approval reason: %+v", status, state)
		}
	}
}

func TestProjectUsesLegacyTopLevelStatusWhenNoRoundExists(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenNeedsRevision
	state := Project(model)
	if state.Kind != KindNeedsOperatorDecision || !state.ApprovalReasonRequired || len(state.Evidence) == 0 {
		t.Fatalf("legacy state = %+v", state)
	}
}

func TestProjectRequiresReasonForLegacyPassedStatusWithoutRound(t *testing.T) {
	t.Parallel()

	model := fixtureModel()
	model.HardenStatus = spec.HardenPassed
	state := Project(model)
	if state.Kind != KindStaleAfterDraft || !state.ApprovalReasonRequired || state.Next != HardenCommand(model.TaskID) {
		t.Fatalf("legacy passed status should not approve without round evidence: %+v", state)
	}
}

func fixtureModel() spec.Model {
	return spec.Model{
		Version: "2.0",
		TaskID:  "task",
		Status:  spec.StatusDraft,
		Title:   "Task",
		Summary: "Original summary",
	}
}
