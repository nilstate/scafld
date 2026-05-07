package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceInitCreatesScafldLayout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := (WorkspaceStore{}).Init(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Root == "" {
		t.Fatal("root not recorded")
	}
	for _, rel := range []string{
		".scafld",
		".scafld/core",
		".scafld/prompts",
		".scafld/specs/drafts",
		".scafld/specs/approved",
		".scafld/specs/active",
		".scafld/runs",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
}
