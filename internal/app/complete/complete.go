package complete

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrReviewGate is returned when completion is attempted before a passing review.
var ErrReviewGate = errors.New("review gate has not passed")

// SpecStore is the spec persistence port used by completion.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
	Save(context.Context, string, spec.Model) error
}

// SessionStore is the session evidence port used by completion.
type SessionStore interface {
	Append(context.Context, string, session.Entry, string) (session.Session, error)
	Load(context.Context, string) (session.Session, error)
}

// Clock supplies completion timestamps.
type Clock interface{ Now() time.Time }

// Run completes a reviewed task and records completion evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, clock Clock, taskID string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return spec.Model{}, reviewGateError(model, "session ledger could not be loaded", "missing review evidence")
	}
	reviewEntry, ok := latestCompletionReviewEntry(ledger)
	if !ok || reviewEntry.Status != corereview.VerdictPass {
		actual := "no current accepted review"
		if ok {
			actual = "latest review verdict " + reviewEntry.Status
		}
		return spec.Model{}, reviewGateError(model, "latest review gate has not passed", actual)
	}
	if !corereview.ValidCompletionProvider(reviewEntry.Provider) {
		return spec.Model{}, reviewGateError(model, "passing review came from an unsupported provider", fmt.Sprintf("provider %q", reviewEntry.Provider))
	}
	if reviewEntry.Output != "" {
		dossier, ok := corereview.DecodeDossier(reviewEntry.Output)
		if !ok {
			return spec.Model{}, reviewGateError(model, "latest review dossier is invalid", "review entry output could not be decoded as ReviewDossier")
		}
		if blockers := corereview.OpenBlockerCount(dossier.Findings); blockers > 0 {
			return spec.Model{}, reviewGateError(model, "latest review dossier still has open completion blockers", fmt.Sprintf("%d open blocker(s)", blockers))
		}
	}
	if reviewEntry.Provider == "human" && !hasAuditedHumanOverride(ledger) {
		return spec.Model{}, reviewGateError(model, "human-reviewed gate is missing audit evidence", "latest human review has no adjacent review_override entry")
	}
	model = reconcile.FromSession(model, ledger)
	if model.Status != spec.StatusReview || model.Review.Verdict != corereview.VerdictPass {
		return spec.Model{}, reviewGateError(model, "projected spec is not at a passing review gate", fmt.Sprintf("status %s verdict %s", model.Status, model.Review.Verdict))
	}
	now := clock.Now().UTC().Format(time.RFC3339)
	ledger, err = sessions.Append(ctx, model.TaskID, session.Entry{Type: "complete", Status: "completed", Reason: "task completed"}, now)
	if err != nil {
		return spec.Model{}, err
	}
	if loaded, loadErr := sessions.Load(ctx, model.TaskID); loadErr == nil {
		ledger = loaded
	}
	model = reconcile.FromSession(model, ledger)
	model.Status = spec.StatusCompleted
	model.Updated = now
	model.CurrentState.Next = "done"
	model.CurrentState.AllowedFollowUp = "none"
	if err := specs.Save(ctx, path, model); err != nil {
		return spec.Model{}, err
	}
	return model, nil
}

func reviewGateError(model spec.Model, reason string, actual string) error {
	return gate.New(ErrReviewGate, gate.Failure{
		Gate:     "complete",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: []string{"session review entries", "projected spec review state"},
		Expected: "latest accepted review verdict pass from codex, claude, command, or audited human provider",
		Actual:   actual,
		Blockers: []string{reason},
		Next:     "scafld review " + model.TaskID,
	})
}

func hasAuditedHumanOverride(ledger session.Session) bool {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		if entry.Provider != "human" {
			return true
		}
		if i == 0 {
			return false
		}
		previous := ledger.Entries[i-1]
		return previous.Type == "review_override" && previous.Provider == "human" && previous.Status == "accepted"
	}
	return false
}

func latestCompletionReviewEntry(ledger session.Session) (session.Entry, bool) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		switch entry.Type {
		case "review":
			return entry, true
		case "review_attempt", "build", "criterion", "phase", "approval", session.EntryWorkspaceBaseline, "fail", "cancel":
			return session.Entry{}, false
		}
	}
	return session.Entry{}, false
}
