package complete

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/reconcile"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewgate"
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

type workspaceMaterialStatus interface {
	MaterialSeal(context.Context, []string) (reviewevidence.MaterialSeal, error)
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
	nowTime := clock.Now().UTC()
	now := nowTime.Format(time.RFC3339)
	opts := reviewgate.Options{Now: nowTime}
	if workspace != nil {
		seal, err := currentWorkspaceSeal(ctx, workspace)
		if err != nil {
			return spec.Model{}, reviewGateError(model, "latest review is stale against current workspace", err.Error())
		}
		opts.WorkspaceSeal = seal
		opts.HasWorkspaceSeal = true
		authority := reviewgate.CurrentReviewGate(ledger)
		if authority.Valid && strings.TrimSpace(authority.ReviewEntry.ReviewedMaterialDigest) != "" && reviewedScopePresent(authority.ReviewEntry.ReviewedScope) {
			material, err := currentMaterialSeal(ctx, workspace, authority.ReviewEntry.ReviewedScope)
			if err != nil {
				return spec.Model{}, reviewGateError(model, "latest review is stale against current task material", err.Error())
			}
			opts.MaterialSeal = material
			opts.HasMaterialSeal = true
		}
	}
	state := reviewgate.Project(ledger, model, opts)
	if state.Kind != reviewgate.KindReviewPassed {
		return spec.Model{}, reviewGateErrorWithNext(model, state.Reason, state.Actual, state.Next)
	}
	model = reconcile.FromSession(model, ledger)
	if !projectedReviewPassed(model) {
		return spec.Model{}, reviewGateError(model, "projected spec is not at a passing review gate", fmt.Sprintf("status %s verdict %s", model.Status, model.Review.Verdict))
	}
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

func currentWorkspaceSeal(ctx context.Context, workspace WorkspaceStatus) (reviewgate.WorkspaceSeal, error) {
	snapshot, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return reviewgate.WorkspaceSeal{}, fmt.Errorf("current workspace snapshot failed: %w", err)
	}
	comparison := reviewevidence.ComparisonSnapshot(snapshot)
	return reviewgate.WorkspaceSeal{
		Head:  currentHead(ctx, workspace),
		Dirty: reviewevidence.SnapshotDirty(comparison),
		Diff:  reviewevidence.SnapshotDigest(comparison),
	}, nil
}

func currentMaterialSeal(ctx context.Context, workspace WorkspaceStatus, scope []string) (reviewevidence.MaterialSeal, error) {
	material, ok := workspace.(workspaceMaterialStatus)
	if !ok {
		return reviewevidence.MaterialSeal{}, fmt.Errorf("workspace material seal is unavailable")
	}
	return material.MaterialSeal(ctx, scope)
}

func reviewedScopePresent(scope []string) bool {
	for _, item := range scope {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
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
	return reviewGateErrorWithNext(model, reason, actual, "")
}

func reviewGateErrorWithNext(model spec.Model, reason string, actual string, projectedNext string) error {
	next := model.CurrentState.AllowedFollowUp
	if projectedNext != "" {
		next = projectedNext
	}
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
