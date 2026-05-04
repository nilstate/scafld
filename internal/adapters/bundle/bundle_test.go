package bundle

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBundleManifestHashVerification(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest, err := Verify(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Hash == "" {
		t.Fatal("hash missing")
	}
}
