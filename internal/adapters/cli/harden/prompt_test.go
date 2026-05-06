package harden

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptIncludesConfiguredHardenCap(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
harden:
  max_questions_per_round: 3
invariants:
  canonical:
    tenant_isolation: "Do not leak data across tenants."
`), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := Prompt(context.Background(), root)
	if !strings.Contains(prompt, "Configured max_questions_per_round: 3") {
		t.Fatalf("prompt missing config cap:\n%s", prompt)
	}
	if !strings.Contains(prompt, "tenant_isolation: Do not leak data across tenants.") {
		t.Fatalf("prompt missing configured invariant:\n%s", prompt)
	}
}
