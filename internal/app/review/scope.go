package review

import (
	"context"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/reviewscope"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

func workspaceSnapshot(ctx context.Context, workspace WorkspaceStatus) ([]string, error) {
	if workspace == nil {
		return nil, nil
	}
	files, err := workspace.ChangedFiles(ctx)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), files...), nil
}

func taskBaseline(ctx context.Context, sessions SessionStore, taskID string, fallback []string) []string {
	if sessions == nil {
		return append([]string(nil), fallback...)
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return append([]string(nil), fallback...)
	}
	entry, ok := session.FirstWorkspaceBaseline(ledger)
	if !ok {
		return append([]string(nil), fallback...)
	}
	return session.WorkspaceBaselineSnapshot(entry)
}

func reviewComparisonSnapshot(snapshot []string) []string {
	return reviewevidence.ComparisonSnapshot(snapshot)
}

func reviewComparisonPath(path string) bool {
	return reviewevidence.ComparisonPath(path)
}

func reviewBlockingMutations(before []string, after []string, scope []string, specPath string) []coreworkspace.Mutation {
	currentSpec := currentSpecReviewPath(specPath)
	normalizedScope := coreworkspace.NormalizeScope(scope)
	var blocking []coreworkspace.Mutation
	for _, mutation := range coreworkspace.Diff(before, after) {
		path := strings.Trim(strings.ReplaceAll(mutation.Path, "\\", "/"), "/")
		if currentSpec != "" && path == currentSpec {
			blocking = append(blocking, mutation)
			continue
		}
		if !reviewComparisonPath(path) {
			continue
		}
		if len(normalizedScope) == 0 || coreworkspace.PathInScope(path, normalizedScope) {
			blocking = append(blocking, mutation)
		}
	}
	return blocking
}

func currentSpecReviewPath(path string) string {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if idx := strings.Index(normalized, ".scafld/specs/"); idx >= 0 {
		return normalized[idx:]
	}
	return ""
}

func reviewAttemptOutput(baseline []string, taskChanges []coreworkspace.Mutation, scopeDrift []coreworkspace.Mutation) string {
	var b strings.Builder
	b.WriteString("baseline:\n")
	for _, path := range coreworkspace.Paths(baseline) {
		fmt.Fprintf(&b, "- %s\n", path)
	}
	b.WriteString("task_changes_since_baseline:\n")
	for _, line := range coreworkspace.MutationStrings(taskChanges) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	b.WriteString("ambient_drift_outside_task_scope:\n")
	for _, line := range coreworkspace.MutationStrings(scopeDrift) {
		fmt.Fprintf(&b, "- %s\n", line)
	}
	return strings.TrimRight(b.String(), "\n")
}

type reviewSessionSeal struct {
	reviewedHead  string
	reviewedDirty string
	reviewedDiff  string
}

type workspaceMaterialStatus interface {
	MaterialSeal(context.Context, []string) (reviewevidence.MaterialSeal, error)
}

func reviewSeal(ctx context.Context, workspace WorkspaceStatus, snapshot []string) (reviewSessionSeal, error) {
	if workspace == nil {
		return reviewSessionSeal{}, fmt.Errorf("workspace status is required")
	}
	head, hasHead, err := workspace.ResolveHead(ctx)
	if err != nil {
		return reviewSessionSeal{}, fmt.Errorf("resolve workspace head: %w", err)
	}
	head = strings.TrimSpace(head)
	if !hasHead || head == "" {
		head = "unborn"
	}
	return reviewSessionSeal{
		reviewedHead:  head,
		reviewedDirty: reviewevidence.SnapshotDirty(snapshot),
		reviewedDiff:  reviewevidence.SnapshotDigest(snapshot),
	}, nil
}

func reviewMaterialScope(scope []string, reviewedSnapshot []string) []string {
	return reviewscope.MaterialScope(scope, reviewedSnapshot)
}

func reviewMaterialSeal(ctx context.Context, workspace WorkspaceStatus, scope []string) (reviewevidence.MaterialSeal, bool, error) {
	scope = coreworkspace.NormalizeScope(scope)
	if len(scope) == 0 {
		return reviewevidence.MaterialSeal{}, false, nil
	}
	material, ok := workspace.(workspaceMaterialStatus)
	if !ok {
		return reviewevidence.MaterialSeal{}, false, nil
	}
	seal, err := material.MaterialSeal(ctx, scope)
	if err != nil {
		return reviewevidence.MaterialSeal{}, false, err
	}
	if strings.TrimSpace(seal.Digest) == "" {
		return reviewevidence.MaterialSeal{}, false, nil
	}
	if len(seal.Scope) == 0 {
		seal.Scope = append([]string(nil), scope...)
	}
	return seal, true, nil
}

func deriveReviewScope(model spec.Model, explicit []string, snapshot []string) []string {
	return reviewscope.Derive(model, explicit, snapshot)
}

func filterReviewScope(scope []string) []string {
	return reviewscope.FilterAllowed(scope)
}
