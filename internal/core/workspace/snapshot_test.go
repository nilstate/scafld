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
