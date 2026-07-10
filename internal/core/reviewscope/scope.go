// Package reviewscope derives the task-owned workspace surface used by review,
// status, handoff, and completion freshness checks.
package reviewscope

import (
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

// Projection is the task-owned workspace surface relative to the captured
// baseline. Baseline and current snapshots are already filtered to the review
// comparison surface, excluding scafld runtime state.
type Projection struct {
	Scope        []string
	Baseline     []string
	Current      []string
	TaskChanges  []coreworkspace.Mutation
	AmbientDrift []coreworkspace.Mutation
}

// Project classifies current workspace changes against the task contract and
// captured baseline.
func Project(model spec.Model, explicit []string, baselineSnapshot []string, currentSnapshot []string) Projection {
	baseline := reviewevidence.ComparisonSnapshot(baselineSnapshot)
	current := reviewevidence.ComparisonSnapshot(currentSnapshot)
	scopeInput := append(append([]string(nil), baseline...), current...)
	scope := Derive(model, explicit, scopeInput)
	taskChanges, ambientDrift := coreworkspace.PartitionMutations(coreworkspace.Diff(baseline, current), scope)
	return Projection{
		Scope:        scope,
		Baseline:     coreworkspace.Filter(baseline, scope),
		Current:      coreworkspace.Filter(current, scope),
		TaskChanges:  taskChanges,
		AmbientDrift: ambientDrift,
	}
}

// Derive returns the path scope implied by a task contract. Explicit scope wins.
func Derive(model spec.Model, explicit []string, snapshot []string) []string {
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
	return FilterAllowed(coreworkspace.NormalizeScope(scope))
}

// MaterialScope returns the content scope that should be sealed for a review.
func MaterialScope(scope []string, reviewedSnapshot []string) []string {
	if normalized := coreworkspace.NormalizeScope(scope); len(normalized) > 0 {
		return normalized
	}
	return FilterAllowed(coreworkspace.Paths(reviewedSnapshot))
}

// FilterAllowed removes private/local paths from a review scope.
func FilterAllowed(scope []string) []string {
	filtered := make([]string, 0, len(scope))
	for _, item := range scope {
		if PathAllowed(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// PathAllowed reports whether a path is safe to include in review scope.
func PathAllowed(path string) bool {
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
