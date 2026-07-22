package approve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/hardengate"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrSpecNotDraft is returned when approval is attempted outside draft state.
var ErrSpecNotDraft = errors.New("approve only operates on drafts")

// ErrApprovalReasonRequired is returned when a draft carries harden evidence
// that needs an explicit operator decision before approval.
var ErrApprovalReasonRequired = errors.New("approval reason required")

// SpecStore is the spec persistence port used by approval.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
	Find(string) (string, error)
}

// SessionStore is the session evidence port used by approval.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
}

// WorkspaceStatus captures dirty workspace state before task execution begins.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
}

// Clock supplies approval timestamps.
type Clock interface{ Now() time.Time }

// Output describes the approved task.
type Output struct {
	TaskID string      `json:"task_id"`
	Status spec.Status `json:"status"`
	Path   string      `json:"path"`
}

// Run approves a draft spec and records approval evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, clock Clock, taskID string, reason string) (Output, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	if model.Status != spec.StatusDraft {
		return Output{}, fmt.Errorf("%w: %s", ErrSpecNotDraft, model.Status)
	}
	reason = strings.TrimSpace(reason)
	hardenState := hardengate.Project(model)
	if hardenState.ApprovalReasonRequired && reason == "" {
		return Output{}, gate.New(ErrApprovalReasonRequired, gate.Failure{
			Gate:     "approve",
			Status:   string(model.Status),
			Reason:   "harden findings require an explicit approval reason",
			Evidence: nonEmptyStrings(hardenState.Evidence, "harden gate state: "+string(hardenState.Kind)),
			Expected: "operator approval reason explaining why harden findings are accepted or rejected",
			Actual:   "missing --reason",
			Blockers: nonEmptyStrings(hardenState.Blockers, hardenState.Reason),
			Next:     hardengate.ApproveCommand(model.TaskID) + " --reason <reason>",
		})
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	if sessions != nil {
		if workspace != nil {
			snapshot, err := workspace.ChangedFiles(ctx)
			if err != nil {
				return Output{}, fmt.Errorf("capture workspace baseline: %w", err)
			}
			_, err = sessions.Append(ctx, model.TaskID, session.Entry{
				Type:   session.EntryWorkspaceBaseline,
				Status: "captured",
				Reason: fmt.Sprintf("workspace baseline captured before approval: %d changed path(s)", len(snapshot)),
				Output: strings.Join(snapshot, "\n"),
			}, now)
			if err != nil {
				return Output{}, fmt.Errorf("append workspace baseline: %w", err)
			}
		}
		if hardenState.ApprovalReasonRequired {
			_, err = sessions.Append(ctx, model.TaskID, session.Entry{
				Type:     "harden_override",
				Status:   "accepted",
				Reason:   reason,
				Provider: "human",
				Output:   hardenOverrideOutput(hardenState),
			}, now)
			if err != nil {
				return Output{}, fmt.Errorf("append harden override: %w", err)
			}
		}
		_, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "approval", Status: "approved", Reason: approvalReason(reason)}, now)
		if err != nil {
			return Output{}, fmt.Errorf("append approval: %w", err)
		}
	}
	if hardenState.ApprovalReasonRequired {
		markHardenOverridden(&model, now, reason)
	}
	model.Status = spec.StatusApproved
	model.CurrentState.Next = "build"
	model.CurrentState.AllowedFollowUp = "scafld build " + model.TaskID
	model.Updated = now
	if err := specs.Save(ctx, path, model); err != nil {
		return Output{}, fmt.Errorf("save approved spec: %w", err)
	}
	path, err = specs.Find(model.TaskID)
	if err != nil {
		return Output{}, fmt.Errorf("find approved spec: %w", err)
	}
	return Output{TaskID: model.TaskID, Status: model.Status, Path: path}, nil
}

func approvalReason(reason string) string {
	if reason == "" {
		return "spec approved"
	}
	return "spec approved: " + reason
}

func markHardenOverridden(model *spec.Model, now string, reason string) {
	model.HardenStatus = spec.HardenOverridden
	if len(model.HardenRounds) == 0 {
		return
	}
	latest := len(model.HardenRounds) - 1
	summary := "operator override: " + reason
	if existing := strings.TrimSpace(model.HardenRounds[latest].Summary); existing != "" {
		summary += "; harden summary: " + existing
	}
	model.HardenRounds[latest].Status = string(spec.HardenOverridden)
	model.HardenRounds[latest].EndedAt = now
	model.HardenRounds[latest].Summary = summary
}

func hardenOverrideOutput(state hardengate.State) string {
	payload := struct {
		Kind          hardengate.Kind `json:"kind"`
		Reason        string          `json:"reason,omitempty"`
		CurrentDigest string          `json:"current_digest,omitempty"`
		RoundID       string          `json:"round_id,omitempty"`
		RoundStatus   string          `json:"round_status,omitempty"`
		SameDraft     bool            `json:"same_draft,omitempty"`
		Evidence      []string        `json:"evidence,omitempty"`
		Blockers      []string        `json:"blockers,omitempty"`
	}{
		Kind:          state.Kind,
		Reason:        state.Reason,
		CurrentDigest: state.CurrentDigest,
		RoundID:       state.LatestRound.ID,
		RoundStatus:   state.LatestRound.Status,
		SameDraft:     state.SameDraft,
		Evidence:      state.Evidence,
		Blockers:      state.Blockers,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return string(state.Kind)
	}
	return string(data)
}

func nonEmptyStrings(values []string, fallback string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	if len(out) == 0 && strings.TrimSpace(fallback) != "" {
		return []string{fallback}
	}
	return out
}
