package status

import (
	"context"

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
	TaskID    string      `json:"task_id"`
	Status    spec.Status `json:"status"`
	Title     string      `json:"title"`
	Next      string      `json:"next"`
	SessionOK bool        `json:"session_ok"`
	Review    ReviewInfo  `json:"review,omitempty"`
}

// ReviewInfo is the latest review evidence visible from status.
type ReviewInfo struct {
	Verdict       string               `json:"verdict,omitempty"`
	Findings      []corereview.Finding `json:"findings,omitempty"`
	Running       bool                 `json:"running,omitempty"`
	AttemptStatus string               `json:"attempt_status,omitempty"`
	Reason        string               `json:"reason,omitempty"`
}

// Run reads status for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (Output, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return Output{}, err
	}
	out := Output{TaskID: model.TaskID, Status: model.Status, Title: model.Title, Next: model.CurrentState.AllowedFollowUp}
	if sessions != nil {
		if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
			out.SessionOK = true
			out.Review = latestReviewInfo(ledger)
		}
	}
	return out, nil
}

func latestReviewInfo(ledger session.Session) ReviewInfo {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			return ReviewInfo{
				Verdict:  entry.Status,
				Findings: corereview.DecodeFindings(entry.Output),
			}
		case "review_attempt":
			info := ReviewInfo{
				Running:       entry.Status == "running",
				AttemptStatus: entry.Status,
				Reason:        entry.Reason,
			}
			return info
		}
	}
	return ReviewInfo{}
}
