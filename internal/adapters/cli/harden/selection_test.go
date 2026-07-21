package harden

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
)

func TestSelectLeavesManualHardenWhenNoProviderConfigured(t *testing.T) {
	t.Parallel()

	selected, err := Select(context.Background(), Options{Root: t.TempDir(), TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != nil {
		t.Fatalf("provider = %T, want manual harden provider nil", selected.Provider)
	}
	if selected.ContextMaxBytes <= 0 {
		t.Fatalf("context max bytes not defaulted: %d", selected.ContextMaxBytes)
	}
}

func TestSelectUsesCodexHardenEffortFromConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
harden:
  external:
    provider: "codex"
    codex:
      model: "latest"
      model_reasoning_effort: "xhigh"
      binary: "codex-config"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task"})
	if err != nil {
		t.Fatal(err)
	}
	hardenProvider, ok := selected.Provider.(providers.HardenProvider)
	if !ok {
		t.Fatalf("provider = %T, want providers.HardenProvider", selected.Provider)
	}
	codex, ok := hardenProvider.Agent.(providers.CodexProvider)
	if !ok {
		t.Fatalf("agent = %T, want CodexProvider", hardenProvider.Agent)
	}
	if codex.Model != "" || codex.ModelReasoningEffort != "xhigh" || codex.Binary != "codex-config" {
		t.Fatalf("codex harden provider did not use config: %+v", codex)
	}
}

func TestSelectUsesGeminiHardenProviderFromConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
harden:
  context_max_bytes: 2048
  required_context_max_bytes: 65536
  external:
    provider: "gemini"
    idle_timeout_seconds: 17
    absolute_max_seconds: 41
    gemini:
      model: "gemini-harden-config"
      binary: "gemini-harden-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress bytes.Buffer
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task", Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	if selected.ContextMaxBytes != 2048 || selected.RequiredContextMaxBytes != 65536 {
		t.Fatalf("context budgets = %d/%d", selected.ContextMaxBytes, selected.RequiredContextMaxBytes)
	}
	hardenProvider, ok := selected.Provider.(providers.HardenProvider)
	if !ok {
		t.Fatalf("provider = %T, want providers.HardenProvider", selected.Provider)
	}
	gemini, ok := hardenProvider.Agent.(providers.GeminiProvider)
	if !ok {
		t.Fatalf("agent = %T, want GeminiProvider", hardenProvider.Agent)
	}
	if gemini.Binary != "gemini-harden-bin" || gemini.Model != "gemini-harden-config" || gemini.Timeout.String() != "41s" || gemini.IdleTimeout.String() != "17s" {
		t.Fatalf("gemini harden provider did not use config: %+v", gemini)
	}
	runner, ok := gemini.Runner.(process.Runner)
	if !ok {
		t.Fatalf("runner = %T, want process.Runner", gemini.Runner)
	}
	if runner.Progress != &progress || runner.ProgressLabel != "harden[gemini:gemini-harden-config]" {
		t.Fatalf("harden runner did not carry Gemini progress label: %+v", runner)
	}

	selected, err = Select(context.Background(), Options{Root: root, TaskID: "task", Model: "gemini-harden-flag"})
	if err != nil {
		t.Fatal(err)
	}
	hardenProvider = selected.Provider.(providers.HardenProvider)
	gemini = hardenProvider.Agent.(providers.GeminiProvider)
	if gemini.Model != "gemini-harden-flag" {
		t.Fatalf("flag model should override Gemini harden config: %+v", gemini)
	}
}
