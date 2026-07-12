package workspace

import (
	"strings"
	"testing"
)

func TestDiffDetectsAddModifyDelete(t *testing.T) {
	t.Parallel()

	mutations := MutationStrings(Diff(
		[]string{" M hash-a same", " M hash-old modified", " D deleted removed"},
		[]string{" M hash-a same", " M hash-new modified", "?? hash-add added"},
	))
	if len(mutations) != 3 {
		t.Fatalf("mutations = %+v", mutations)
	}
	joined := strings.Join(mutations, "\n")
	for _, want := range []string{"added added", "changed modified", "removed removed"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("mutations missing %q: %+v", want, mutations)
		}
	}
}

func TestFilterAndPartitionUsePathPrefixes(t *testing.T) {
	t.Parallel()

	scope := NormalizeScope([]string{"./api/", "api", "cli/packages/mcp", "docs/**"})
	if len(scope) != 3 || scope[0] != "api" || scope[1] != "cli/packages/mcp" || scope[2] != "docs" {
		t.Fatalf("scope = %+v", scope)
	}
	filtered := Filter([]string{" M a api/handler.go", " M b docs/index.md"}, scope)
	if len(filtered) != 2 {
		t.Fatalf("filtered = %+v", filtered)
	}
	inside, outside := PartitionMutations([]Mutation{{Kind: "changed", Path: "api/handler.go"}, {Kind: "added", Path: "docs/index.md"}}, scope)
	if len(inside) != 2 || len(outside) != 0 {
		t.Fatalf("inside=%+v outside=%+v", inside, outside)
	}
}

func TestPathInScopeRootScopeMatchesEveryPath(t *testing.T) {
	scope := NormalizeScope([]string{"."})
	for _, path := range []string{"cmd/main.go", "internal/core/workspace/snapshot.go", "README.md"} {
		if !PathInScope(path, scope) {
			t.Fatalf("PathInScope(%q, %q) = false, want true: a \".\" scope denotes the workspace root", path, scope)
		}
	}
}

func TestFilterRootScopeKeepsEveryEntry(t *testing.T) {
	snapshot := []string{"M cmd/main.go", "A internal/core/x.go"}
	filtered := Filter(snapshot, NormalizeScope([]string{"."}))
	if len(filtered) != len(snapshot) {
		t.Fatalf("Filter(%q, [\".\"]) = %q, want every entry retained", snapshot, filtered)
	}
}

func TestPathInScopeIgnoresLeadingDotSlashOnCandidate(t *testing.T) {
	scope := NormalizeScope([]string{"./cmd"})
	if !PathInScope("./cmd/main.go", scope) {
		t.Fatalf("PathInScope(\"./cmd/main.go\", %q) = false, want true: candidate normalization must match NormalizeScope", scope)
	}
}
