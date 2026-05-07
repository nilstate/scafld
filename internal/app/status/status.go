package status

import (
	"context"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec loading port used by status.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore is the session loading port used by status.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// Output describes the task status projection.
type Output struct {
	TaskID          string        `json:"task_id"`
	Status          spec.Status   `json:"status"`
	Title           string        `json:"title"`
	Next            string        `json:"next"`
	Gate            string        `json:"gate,omitempty"`
	TrustedState    string        `json:"trusted_state,omitempty"`
	AllowedFollowUp string        `json:"allowed_follow_up,omitempty"`
	SessionOK       bool          `json:"session_ok"`
	Repair          *gate.Failure `json:"repair,omitempty"`
	Review          ReviewInfo    `json:"review,omitempty"`
}

// ReviewInfo is the latest review evidence visible from status.
type ReviewInfo struct {
	Verdict       string               `json:"verdict,omitempty"`
	Findings      []corereview.Finding `json:"findings,omitempty"`
	Attempt       *ReviewAttemptInfo   `json:"attempt,omitempty"`
	Running       bool                 `json:"running,omitempty"`
	AttemptStatus string               `json:"attempt_status,omitempty"`
	Reason        string               `json:"reason,omitempty"`
}

// ReviewAttemptInfo describes the latest provider attempt separately from the
// latest accepted review verdict.
type ReviewAttemptInfo struct {
	Running bool   `json:"running"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

// Run reads status for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (Output, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	out := Output{
		TaskID:          model.TaskID,
		Status:          model.Status,
		Title:           model.Title,
		Next:            model.CurrentState.AllowedFollowUp,
		Gate:            currentGate(model),
		TrustedState:    "session ledger replay projected into the Markdown spec",
		AllowedFollowUp: model.CurrentState.AllowedFollowUp,
	}
	if sessions != nil {
		if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
			out.SessionOK = true
			out.Review = latestReviewInfo(ledger)
			out.Repair = repairContract(model, ledger)
		}
	}
	return out, nil
}

func latestReviewInfo(ledger session.Session) ReviewInfo {
	var info ReviewInfo
	haveAttempt := false
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			info.Verdict = entry.Status
			info.Findings = corereview.DecodeFindings(entry.Output)
			return info
		case "review_attempt":
			if haveAttempt {
				continue
			}
			info.Running = entry.Status == "running"
			info.AttemptStatus = entry.Status
			info.Reason = entry.Reason
			info.Attempt = &ReviewAttemptInfo{Running: info.Running, Status: entry.Status, Reason: entry.Reason}
			haveAttempt = true
		}
	}
	return info
}

func currentGate(model spec.Model) string {
	switch model.Status {
	case spec.StatusDraft:
		if model.HardenStatus == spec.HardenInProgress {
			return "harden"
		}
		return "approve"
	case spec.StatusApproved, spec.StatusActive, spec.StatusBlocked:
		return "build"
	case spec.StatusReview:
		return "review"
	case spec.StatusCompleted:
		return "complete"
	default:
		return string(model.Status)
	}
}

func repairContract(model spec.Model, ledger session.Session) *gate.Failure {
	switch model.Status {
	case spec.StatusBlocked:
		blockers := criterionBlockers(model)
		return &gate.Failure{
			Gate:     "build",
			Status:   string(model.Status),
			Reason:   model.CurrentState.Reason,
			Evidence: blockerEvidence(model, ledger),
			Expected: "all acceptance criteria pass",
			Actual:   fmt.Sprintf("%d blocker(s)", len(blockers)),
			Blockers: blockers,
			Next:     model.CurrentState.AllowedFollowUp,
		}
	case spec.StatusReview:
		review := latestReviewInfo(ledger)
		if review.Verdict == corereview.VerdictFail {
			return &gate.Failure{
				Gate:     "review",
				Status:   string(model.Status),
				Reason:   "review verdict fail",
				Evidence: []string{latestReviewEvidence(ledger)},
				Expected: "review verdict pass",
				Actual:   "review verdict fail",
				Blockers: reviewFindingSummaries(review.Findings),
				Next:     model.CurrentState.AllowedFollowUp,
			}
		}
	}
	return nil
}

func latestReviewEvidence(ledger session.Session) string {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		if entry.ID != "" {
			return entry.ID
		}
		return "review event"
	}
	return "review event"
}

func criterionBlockers(model spec.Model) []string {
	var blockers []string
	for _, criterion := range model.AllCriteria() {
		if criterion.Status == "pass" {
			continue
		}
		label := criterion.ID
		if criterion.Title != "" {
			label += ": " + criterion.Title
		}
		if criterion.Evidence != "" {
			label += " (" + criterion.Evidence + ")"
		}
		blockers = append(blockers, label)
	}
	return blockers
}

func blockerEvidence(model spec.Model, ledger session.Session) []string {
	var evidence []string
	for _, criterion := range model.AllCriteria() {
		if criterion.Status == "pass" || criterion.SourceEvent == "" {
			continue
		}
		evidence = append(evidence, evidenceReference(ledger, criterion.SourceEvent))
	}
	return evidence
}

func evidenceReference(ledger session.Session, sourceID string) string {
	for _, entry := range ledger.Entries {
		if entry.ID != sourceID {
			continue
		}
		if entry.Path != "" {
			return entry.Path
		}
		return sourceID
	}
	return sourceID
}

func reviewFindingSummaries(findings []corereview.Finding) []string {
	blockers := make([]string, 0, len(findings))
	for _, finding := range findings {
		if finding.Severity != corereview.SeverityBlocking {
			continue
		}
		label := finding.ID
		if finding.Summary != "" {
			label += ": " + finding.Summary
		}
		blockers = append(blockers, label)
	}
	return blockers
}
