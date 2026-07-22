package harden

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptFramesHardenBudgetAsRealFindingsNotFiller(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
harden:
  max_issues_per_round: 3
`), 0o644); err != nil {
		t.Fatal(err)
	}
	prompt := Prompt(context.Background(), root)
	for _, want := range []string{
		"draft as a hypothesis",
		"not evidence that the proposed",
		"reject` / no-op",
		"reuse or move ownership",
		"materially better shape",
		"report shrink or\nreframe",
		"code-shape and system-design gate",
		"not coverage bookkeeping",
		"consumer-by-consumer compliance matrix",
		"bespoke test for every surface",
		"Per-surface implementation correctness belongs to build evidence and review",
		"finding as many real spec issues",
		"budget for real findings, not filler",
		"right-to-exist",
		"shared core/app contract",
		"API, MCP, CLI",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
