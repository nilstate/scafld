package workspace

import (
	"path/filepath"
	"testing"
)

func TestEvidenceRootsNormalizesAndDedupeRoots(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "api")
	got := EvidenceRoots(root, []string{"..", "../app", "../app"})
	want := []string{
		filepath.Clean(root),
		filepath.Clean(filepath.Join(root, "..")),
		filepath.Clean(filepath.Join(root, "..", "app")),
	}
	if len(got) != len(want) {
		t.Fatalf("roots = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roots = %#v, want %#v", got, want)
		}
	}
	if !InsideAnyRoot(filepath.Join(root, "..", "app", "src", "view.ts"), got) {
		t.Fatalf("expected sibling app path inside roots: %#v", got)
	}
	if InsideAnyRoot(filepath.Join(root, "..", "..", "outside.txt"), got) {
		t.Fatalf("unexpected outside path inside roots: %#v", got)
	}
}
