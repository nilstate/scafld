package corebundle

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type assetFile struct {
	data []byte
	exec bool
}

func TestEmbeddedAssetsDoNotDriftFromWorkspaceAssets(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertAssetTreeMatches(t, filepath.Join(root, ".scafld", "core"), filepath.Join(root, "internal", "adapters", "corebundle", "assets", "core"), ".scafld/core")
	assertAssetTreeMatches(t, filepath.Join(root, ".scafld", "prompts"), filepath.Join(root, "internal", "adapters", "corebundle", "assets", "prompts"), ".scafld/prompts")
}

func assertAssetTreeMatches(t *testing.T, canonicalRoot string, embeddedRoot string, label string) {
	t.Helper()

	canonical := collectAssetTree(t, canonicalRoot)
	embedded := collectAssetTree(t, embeddedRoot)

	for rel, want := range canonical {
		got, ok := embedded[rel]
		if !ok {
			t.Fatalf("%s drift: embedded bundle is missing %s; sync %s into %s", label, rel, canonicalRoot, embeddedRoot)
		}
		if !bytes.Equal(got.data, want.data) {
			t.Fatalf("%s drift: %s differs between %s and %s", label, rel, canonicalRoot, embeddedRoot)
		}
		if got.exec != want.exec {
			t.Fatalf("%s drift: %s executable bit differs between %s and %s", label, rel, canonicalRoot, embeddedRoot)
		}
	}

	for rel := range embedded {
		if _, ok := canonical[rel]; !ok {
			t.Fatalf("%s drift: embedded bundle has extra file %s; remove it or add the canonical file under %s", label, rel, canonicalRoot)
		}
	}
}

func collectAssetTree(t *testing.T, root string) map[string]assetFile {
	t.Helper()

	files := make(map[string]assetFile)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("stat asset root %s: %v", root, err)
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = assetFile{
			data: data,
			exec: info.Mode()&0o111 != 0,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk asset root %s: %v", root, err)
	}
	return files
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", file)
		}
		dir = parent
	}
}
