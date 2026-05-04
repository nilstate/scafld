package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplaceCleanup(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file.txt")
	if err := Write(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("data = %q", data)
	}
}
