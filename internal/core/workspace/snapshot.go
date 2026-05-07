package workspace

import (
	"fmt"
	"sort"
	"strings"
)

// Change is one fingerprinted workspace path from a source-control adapter.
type Change struct {
	Raw         string
	Status      string
	Fingerprint string
	Path        string
}

// Mutation describes one path-level difference between two workspace snapshots.
type Mutation struct {
	Kind   string
	Path   string
	Before Change
	After  Change
}

// String renders a stable, human-readable mutation summary.
func (m Mutation) String() string {
	switch m.Kind {
	case "added":
		return fmt.Sprintf("added %s (%s)", m.Path, m.After.State())
	case "removed":
		return fmt.Sprintf("removed %s (was %s)", m.Path, m.Before.State())
	default:
		return fmt.Sprintf("changed %s (%s -> %s)", m.Path, m.Before.State(), m.After.State())
	}
}

// State returns a compact status+fingerprint value for diagnostics.
func (c Change) State() string {
	if c.Fingerprint == "" {
		return c.Status
	}
	return strings.TrimSpace(c.Status + " " + c.Fingerprint)
}

// ParseChange parses the raw line format emitted by workspace adapters.
func ParseChange(raw string) Change {
	text := strings.TrimSpace(raw)
	if len(text) < 3 {
		return Change{Raw: raw, Status: "changed", Path: text}
	}
	status := strings.TrimSpace(text[:2])
	if status == "" {
		status = "changed"
	}
	rest := strings.TrimSpace(text[2:])
	fingerprint, path, ok := strings.Cut(rest, " ")
	if !ok || strings.TrimSpace(path) == "" {
		return Change{Raw: raw, Status: status, Path: text}
	}
	return Change{
		Raw:         raw,
		Status:      status,
		Fingerprint: fingerprint,
		Path:        strings.TrimSpace(path),
	}
}

// ChangesByPath indexes a raw snapshot by changed path.
func ChangesByPath(snapshot []string) map[string]Change {
	entries := map[string]Change{}
	for _, raw := range snapshot {
		entry := ParseChange(raw)
		entries[entry.Path] = entry
	}
	return entries
}

// Diff returns stable path-level mutations from before to after.
func Diff(before []string, after []string) []Mutation {
	beforeByPath := ChangesByPath(before)
	afterByPath := ChangesByPath(after)
	paths := map[string]bool{}
	for path := range beforeByPath {
		paths[path] = true
	}
	for path := range afterByPath {
		paths[path] = true
	}

	var changed []Mutation
	for path := range paths {
		beforeEntry, hadBefore := beforeByPath[path]
		afterEntry, hasAfter := afterByPath[path]
		switch {
		case !hadBefore && hasAfter:
			changed = append(changed, Mutation{Kind: "added", Path: path, After: afterEntry})
		case hadBefore && !hasAfter:
			changed = append(changed, Mutation{Kind: "removed", Path: path, Before: beforeEntry})
		case hadBefore && hasAfter && beforeEntry.Raw != afterEntry.Raw:
			changed = append(changed, Mutation{Kind: "changed", Path: path, Before: beforeEntry, After: afterEntry})
		}
	}
	sort.Slice(changed, func(i, j int) bool {
		if changed[i].Path == changed[j].Path {
			return changed[i].Kind < changed[j].Kind
		}
		return changed[i].Path < changed[j].Path
	})
	return changed
}

// MutationStrings renders mutations in deterministic order.
func MutationStrings(mutations []Mutation) []string {
	out := make([]string, 0, len(mutations))
	for _, mutation := range mutations {
		out = append(out, mutation.String())
	}
	return out
}

// Paths returns sorted paths from a raw snapshot.
func Paths(snapshot []string) []string {
	paths := make([]string, 0, len(snapshot))
	for _, raw := range snapshot {
		path := strings.TrimSpace(ParseChange(raw).Path)
		if path != "" {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

// NormalizeScope canonicalizes workspace-relative path prefixes.
func NormalizeScope(scope []string) []string {
	seen := map[string]bool{}
	var normalized []string
	for _, value := range scope {
		text := strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
		text = strings.TrimPrefix(text, "./")
		text = strings.Trim(text, "/")
		text = strings.TrimSuffix(text, "/**")
		text = strings.TrimSuffix(text, "/*")
		text = strings.Trim(text, "/")
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		normalized = append(normalized, text)
	}
	sort.Strings(normalized)
	return normalized
}

// PathInScope reports whether path is equal to or under one of the scope prefixes.
func PathInScope(path string, scope []string) bool {
	candidate := strings.Trim(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"), "/")
	for _, prefix := range scope {
		if candidate == prefix || strings.HasPrefix(candidate, prefix+"/") {
			return true
		}
	}
	return false
}

// Filter returns only snapshot entries under scope. Empty scope keeps all entries.
func Filter(snapshot []string, scope []string) []string {
	if len(scope) == 0 {
		return append([]string(nil), snapshot...)
	}
	var filtered []string
	for _, raw := range snapshot {
		if PathInScope(ParseChange(raw).Path, scope) {
			filtered = append(filtered, raw)
		}
	}
	return filtered
}

// PartitionMutations splits mutations into inside-scope and outside-scope sets.
func PartitionMutations(mutations []Mutation, scope []string) ([]Mutation, []Mutation) {
	if len(scope) == 0 {
		return append([]Mutation(nil), mutations...), nil
	}
	var inside []Mutation
	var outside []Mutation
	for _, mutation := range mutations {
		if PathInScope(mutation.Path, scope) {
			inside = append(inside, mutation)
		} else {
			outside = append(outside, mutation)
		}
	}
	return inside, outside
}
