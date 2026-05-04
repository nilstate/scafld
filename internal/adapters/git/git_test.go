package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestChangedFilesFingerprintsContentChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := (Adapter{Root: root}).ChangedFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := (Adapter{Root: root}).ChangedFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || len(after) != 1 || before[0] == after[0] {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
}
