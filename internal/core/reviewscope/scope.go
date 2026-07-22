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
	mutations := coreworkspace.Diff(baseline, current)
	scope := Derive(model, explicit, scopeInput)
	if len(scope) == 0 {
		scope = mutationScope(mutations)
	}
	taskChanges, ambientDrift := coreworkspace.PartitionMutations(mutations, scope)
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
		return FilterAllowed(normalized)
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

// Literal cleans authoritative git path lists such as scope hints or base diffs.
// It does not require prose-style path evidence, so top-level extensionless
// files such as Makefile, Dockerfile, and LICENSE remain valid material scope.
func Literal(paths []string) []string {
	return FilterAllowed(coreworkspace.NormalizeScope(paths))
}

// Merge returns the sorted, de-duplicated union of scope path lists.
func Merge(a, b []string) []string {
	return FilterAllowed(coreworkspace.NormalizeScope(append(append([]string(nil), a...), b...)))
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

func mutationScope(mutations []coreworkspace.Mutation) []string {
	paths := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		paths = append(paths, mutation.Path)
	}
	return Literal(paths)
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
	value = strings.TrimSpace(value)
	var tokens []string
	text := value
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
	if lower := strings.ToLower(value); strings.HasPrefix(lower, "in scope:") || strings.HasPrefix(lower, "out of scope:") {
		return nil
	}
	if before, _, ok := strings.Cut(value, " - "); ok {
		value = before
	}
	if before, _, ok := strings.Cut(value, ": "); ok {
		value = before
	}
	if before, _, ok := strings.Cut(value, " ("); ok {
		value = before
	}
	for _, part := range strings.Split(value, ",") {
		token := strings.TrimSpace(part)
		token = strings.Trim(strings.ReplaceAll(token, "\\", "/"), "/")
		token = strings.TrimPrefix(token, "./")
		token = strings.Trim(token, "`:,;")
		if looksLikePath(token) {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func looksLikePath(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" || strings.Contains(text, "://") || strings.ContainsAny(text, " \t\n\r") {
		return false
	}
	return text == "." || strings.Contains(text, "/") || strings.HasPrefix(text, ".") || strings.Contains(text, ".") || strings.ContainsAny(text, "*?[")
}
