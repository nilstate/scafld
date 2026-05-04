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
		".scafld/config.yaml",
		".scafld/core/manifest.json",
		".scafld/specs/drafts",
		".scafld/runs",
		".scafld/reviews",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".ai")); !os.IsNotExist(err) {
		t.Fatalf(".ai should not exist after init, stat error = %v", err)
	}
}
