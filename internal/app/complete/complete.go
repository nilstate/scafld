package complete

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corecompletion "github.com/nilstate/scafld/v2/internal/core/completion"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
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

// WorkspaceStatus captures the current Git-visible workspace state for stale
// review checks at completion time.
type WorkspaceStatus interface {
	ChangedFiles(context.Context) ([]string, error)
	ResolveHead(context.Context) (string, bool, error)
}

// Clock supplies completion timestamps.
type Clock interface{ Now() time.Time }

// Run completes a reviewed task and records completion evidence.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, workspace WorkspaceStatus, clock Clock, taskID string) (spec.Model, error) {
	model, path, err := specs.Load(ctx, taskID)
	if err != nil {
		return spec.Model{}, err
	}
	ledger, err := sessions.Load(ctx, model.TaskID)
	if err != nil {
		return spec.Model{}, reviewGateError(model, "session ledger could not be loaded", "missing review evidence")
	}
	authority := corecompletion.CurrentReviewGate(ledger)
	if !authority.Valid {
		return spec.Model{}, reviewGateError(model, authority.Reason, authority.Actual)
	}
	if err := validateCurrentSpecSeal(model, authority); err != nil {
		return spec.Model{}, reviewGateError(model, "latest review is stale against current spec", err.Error())
	}
	if err := validateCurrentWorkspaceSeal(ctx, workspace, authority); err != nil {
		return spec.Model{}, reviewGateError(model, "latest review is stale against current workspace", err.Error())
	}
	model = reconcile.FromSession(model, ledger)
	if !projectedReviewPassed(model) {
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

func projectedReviewPassed(model spec.Model) bool {
	return model.Status == spec.StatusReview && model.Review.Verdict == corereview.VerdictPass
}

func validateCurrentSpecSeal(model spec.Model, authority corecompletion.Authority) error {
	if !authority.Valid || authority.HumanReviewed || authority.ReviewEntry.Type != "review" {
		return nil
	}
	recorded := strings.TrimSpace(authority.ReviewEntry.ReviewedSpec)
	if recorded == "" {
		return fmt.Errorf("reviewed_spec is missing")
	}
	current := spec.ContractDigest(model)
	if recorded != current {
		return fmt.Errorf("reviewed_spec %s, current %s", recorded, current)
	}
	return nil
}

func validateCurrentWorkspaceSeal(ctx context.Context, workspace WorkspaceStatus, authority corecompletion.Authority) error {
	if workspace == nil || !authority.Valid || authority.HumanReviewed || authority.ReviewEntry.Type != "review" {
		return nil
	}
	snapshot, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return fmt.Errorf("current workspace snapshot failed: %w", err)
	}
	current := reviewSeal{
		head:  currentHead(ctx, workspace),
		dirty: reviewevidence.SnapshotDirty(reviewevidence.ComparisonSnapshot(snapshot)),
		diff:  reviewevidence.SnapshotDigest(reviewevidence.ComparisonSnapshot(snapshot)),
	}
	recorded := reviewSeal{
		head:  authority.ReviewEntry.ReviewedHead,
		dirty: authority.ReviewEntry.ReviewedDirty,
		diff:  authority.ReviewEntry.ReviewedDiff,
	}
	if recorded.head != current.head {
		return fmt.Errorf("reviewed_head %s, current %s", recorded.head, current.head)
	}
	if recorded.dirty != current.dirty {
		return fmt.Errorf("reviewed_dirty %s, current %s", recorded.dirty, current.dirty)
	}
	if recorded.diff != current.diff {
		return fmt.Errorf("reviewed_diff %s, current %s", recorded.diff, current.diff)
	}
	return nil
}

type reviewSeal struct {
	head  string
	dirty string
	diff  string
}

func currentHead(ctx context.Context, workspace WorkspaceStatus) string {
	head, hasHead, err := workspace.ResolveHead(ctx)
	if err != nil {
		return "error:" + singleLine(err.Error())
	}
	if !hasHead || strings.TrimSpace(head) == "" {
		return "unborn"
	}
	return strings.TrimSpace(head)
}

func singleLine(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.Join(strings.Fields(value), " ")
}

func reviewGateError(model spec.Model, reason string, actual string) error {
	next := model.CurrentState.AllowedFollowUp
	if next == "scafld complete "+model.TaskID {
		next = "scafld handoff " + model.TaskID
	}
	if next == "" || next == "none" {
		next = "scafld review " + model.TaskID
	}
	return gate.New(ErrReviewGate, gate.Failure{
		Gate:     "complete",
		Status:   string(model.Status),
		Reason:   reason,
		Evidence: []string{"session review entries", "projected spec review state"},
		Expected: "latest accepted review verdict pass from codex, claude, gemini, command, or audited human provider",
		Actual:   actual,
		Blockers: []string{reason},
		Next:     next,
	})
}
