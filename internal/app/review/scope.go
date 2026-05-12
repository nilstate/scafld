package review

import (
	"context"
	"fmt"
	"strings"

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
	var kept []string
	for _, raw := range snapshot {
		if reviewComparisonPath(coreworkspace.ParseChange(raw).Path) {
			kept = append(kept, raw)
		}
	}
	return kept
}

func reviewComparisonPath(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, prefix := range []string{
		".scafld/runs/",
		".scafld/specs/",
	} {
		if strings.HasPrefix(normalized+"/", prefix) {
			return false
		}
	}
	return true
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

func deriveReviewScope(model spec.Model, explicit []string, snapshot []string) []string {
	if normalized := coreworkspace.NormalizeScope(explicit); len(normalized) > 0 {
		return normalized
	}
	var scope []string
	for _, pkg := range model.Context.Packages {
		if looksLikePath(pkg) || packageMatchesWorkspace(pkg, snapshot) {
			scope = append(scope, pkg)
		}
	}
	scope = append(scope, pathishItems(model.Context.FilesImpacted)...)
	scope = append(scope, pathishItems(model.Context.RelatedDocs)...)
	scope = append(scope, pathishItems(model.Scope)...)
	scope = append(scope, pathishItems(model.Touchpoints)...)
	for _, phase := range model.Phases {
		scope = append(scope, pathishItems(phase.Changes)...)
	}
	return filterReviewScope(coreworkspace.NormalizeScope(scope))
}

func filterReviewScope(scope []string) []string {
	filtered := make([]string, 0, len(scope))
	for _, item := range scope {
		if reviewScopePathAllowed(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func reviewScopePathAllowed(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	if normalized == "" {
		return false
	}
	for _, segment := range strings.Split(normalized, "/") {
		if strings.HasPrefix(segment, ".env") {
			return false
		}
	}
	for _, denied := range []string{
		".git",
		".priv",
		".scafld/config.local.yaml",
		".scafld/reviews",
	} {
		if normalized == denied || strings.HasPrefix(normalized+"/", denied+"/") {
			return false
		}
	}
	return true
}

func packageMatchesWorkspace(pkg string, snapshot []string) bool {
	prefixes := coreworkspace.NormalizeScope([]string{pkg})
	if len(prefixes) == 0 {
		return false
	}
	for _, raw := range snapshot {
		if coreworkspace.PathInScope(coreworkspace.ParseChange(raw).Path, prefixes) {
			return true
		}
	}
	return false
}

func pathishItems(values []string) []string {
	var paths []string
	for _, value := range values {
		paths = append(paths, pathishTokens(value)...)
	}
	return paths
}

func pathishTokens(value string) []string {
	var tokens []string
	text := strings.TrimSpace(value)
	for {
		start := strings.Index(text, "`")
		if start < 0 {
			break
		}
		rest := text[start+1:]
		end := strings.Index(rest, "`")
		if end < 0 {
			break
		}
		token := strings.TrimSpace(rest[:end])
		if looksLikePath(token) {
			tokens = append(tokens, token)
		}
		text = rest[end+1:]
	}
	if len(tokens) > 0 {
		return tokens
	}
	first := strings.Fields(strings.TrimLeft(value, "-* "))
	if len(first) == 0 {
		return nil
	}
	token := strings.Trim(first[0], "`:,;")
	if looksLikePath(token) {
		return []string{token}
	}
	return nil
}

func looksLikePath(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" || strings.Contains(text, "://") {
		return false
	}
	return strings.Contains(text, "/") || strings.HasPrefix(text, ".") || strings.Contains(text, ".")
}
