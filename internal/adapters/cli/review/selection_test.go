package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/adapters/providers"
)

func TestSelectUsesConfigAndFlagsOverrideModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  external:
    provider: "codex"
    idle_timeout_seconds: 7
    absolute_max_seconds: 11
    codex:
      model: "gpt-config"
      binary: "codex-config"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	codex, ok := selected.Provider.(providers.CodexProvider)
	if !ok {
		t.Fatalf("provider = %T, want codex", selected.Provider)
	}
	if codex.Model != "gpt-config" || codex.Binary != "codex-config" || codex.Timeout.String() != "11s" || codex.IdleTimeout.String() != "7s" {
		t.Fatalf("codex provider did not use config: %+v", codex)
	}
	if len(selected.Passes) == 0 {
		t.Fatalf("review passes should be supplied from config defaults")
	}
	selected, err = Select(context.Background(), Options{Root: root, TaskID: "task", Model: "gpt-flag"})
	if err != nil {
		t.Fatal(err)
	}
	codex = selected.Provider.(providers.CodexProvider)
	if codex.Model != "gpt-flag" {
		t.Fatalf("flag model should override config: %+v", codex)
	}
}

func TestSelectAutoFailsClosedWhenNoExternalProviderExists(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := Select(context.Background(), Options{Root: t.TempDir(), TaskID: "task"})
	if err == nil || !strings.Contains(err.Error(), "no external review provider found") {
		t.Fatalf("auto provider err = %v", err)
	}
	selected, err := Select(context.Background(), Options{Root: t.TempDir(), TaskID: "task", Provider: "local"})
	if err != nil || selected.Provider == nil {
		t.Fatalf("local provider should remain explicit escape hatch: provider=%T err=%v", selected.Provider, err)
	}
}
