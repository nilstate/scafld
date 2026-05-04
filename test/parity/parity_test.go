package parity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParity(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(filepath.Join("..", "..", "docs", "parity-report.md")); err != nil {
		t.Fatal(err)
	}
}
