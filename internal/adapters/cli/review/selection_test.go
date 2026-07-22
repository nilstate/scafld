package review

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/adapters/process"
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
      model_reasoning_effort: "xhigh"
      binary: "codex-config"
invariants:
  canonical:
    tenant_isolation: "Never leak tenant data."
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
	if codex.Model != "gpt-config" || codex.ModelReasoningEffort != "xhigh" || codex.Binary != "codex-config" || codex.Timeout.String() != "11s" || codex.IdleTimeout.String() != "7s" {
		t.Fatalf("codex provider did not use config: %+v", codex)
	}
	if len(selected.Passes) == 0 {
		t.Fatalf("review passes should be supplied from config defaults")
	}
	if selected.Invariants["tenant_isolation"] != "Never leak tenant data." {
		t.Fatalf("configured invariants missing from selection: %+v", selected.Invariants)
	}
	var progress bytes.Buffer
	selected, err = Select(context.Background(), Options{Root: root, TaskID: "task", Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	codex = selected.Provider.(providers.CodexProvider)
	runner, ok := codex.Runner.(process.Runner)
	if !ok {
		t.Fatalf("runner = %T, want process.Runner", codex.Runner)
	}
	if runner.Progress != &progress || runner.ProgressLabel != "review[codex:gpt-config]" {
		t.Fatalf("review runner did not carry progress stream: %+v", runner)
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
	if err == nil || !strings.Contains(err.Error(), "no external provider found") {
		t.Fatalf("auto provider err = %v", err)
	}
	selected, err := Select(context.Background(), Options{Root: t.TempDir(), TaskID: "task", Provider: "local"})
	if err != nil || selected.Provider == nil {
		t.Fatalf("local provider should remain explicit escape hatch: provider=%T err=%v", selected.Provider, err)
	}
}

func TestSelectAutoPrefersOppositeHostAgent(t *testing.T) {
	t.Setenv("SCAFLD_HOST_AGENT", "codex")

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  external:
    provider: "auto"
    codex:
      model: "gpt-config"
      binary: "codex-config"
    claude:
      model: "claude-config"
      effort: "xhigh"
      binary: "claude-config"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress bytes.Buffer
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task", Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	claude, ok := selected.Provider.(providers.ClaudeProvider)
	if !ok {
		t.Fatalf("provider = %T, want claude opposite Codex host", selected.Provider)
	}
	if claude.Binary != "claude-config" || claude.Model != "claude-config" || claude.Effort != "xhigh" {
		t.Fatalf("claude provider did not use config: %+v", claude)
	}
	runner, ok := claude.Runner.(process.Runner)
	if !ok {
		t.Fatalf("runner = %T, want process.Runner", claude.Runner)
	}
	if runner.ProgressLabel != "review[claude:claude-config]" {
		t.Fatalf("progress label = %q", runner.ProgressLabel)
	}
}

func TestSelectAutoDefaultClaudeModelIsUnpinned(t *testing.T) {
	t.Setenv("SCAFLD_HOST_AGENT", "codex")

	binDir := t.TempDir()
	claudeBin := filepath.Join(binDir, "claude")
	if err := os.WriteFile(claudeBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	var progress bytes.Buffer
	selected, err := Select(context.Background(), Options{Root: t.TempDir(), TaskID: "task", Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	claude, ok := selected.Provider.(providers.ClaudeProvider)
	if !ok {
		t.Fatalf("provider = %T, want claude opposite Codex host", selected.Provider)
	}
	if claude.Model != "" {
		t.Fatalf("default Claude model should be unpinned, got %q", claude.Model)
	}
	runner, ok := claude.Runner.(process.Runner)
	if !ok {
		t.Fatalf("runner = %T, want process.Runner", claude.Runner)
	}
	if runner.ProgressLabel != "review[claude]" {
		t.Fatalf("progress label = %q", runner.ProgressLabel)
	}
}

func TestSelectUsesGeminiProviderFromConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  external:
    provider: "gemini"
    idle_timeout_seconds: 13
    absolute_max_seconds: 37
    gemini:
      model: "gemini-config"
      binary: "gemini-config-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	var progress bytes.Buffer
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task", Progress: &progress})
	if err != nil {
		t.Fatal(err)
	}
	gemini, ok := selected.Provider.(providers.GeminiProvider)
	if !ok {
		t.Fatalf("provider = %T, want GeminiProvider", selected.Provider)
	}
	if gemini.Binary != "gemini-config-bin" || gemini.Model != "gemini-config" || gemini.Timeout.String() != "37s" || gemini.IdleTimeout.String() != "13s" {
		t.Fatalf("gemini provider did not use config: %+v", gemini)
	}
	runner, ok := gemini.Runner.(process.Runner)
	if !ok {
		t.Fatalf("runner = %T, want process.Runner", gemini.Runner)
	}
	if runner.Progress != &progress || runner.ProgressLabel != "review[gemini:gemini-config]" {
		t.Fatalf("review runner did not carry Gemini progress label: %+v", runner)
	}

	selected, err = Select(context.Background(), Options{Root: root, TaskID: "task", Model: "gemini-flag"})
	if err != nil {
		t.Fatal(err)
	}
	gemini = selected.Provider.(providers.GeminiProvider)
	if gemini.Model != "gemini-flag" {
		t.Fatalf("flag model should override Gemini config: %+v", gemini)
	}
}

func TestSelectPrintContextLoadsConfiguredFilesAndSkipsPrivateInputs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  context:
    max_bytes: 4096
    required_max_bytes: 65536
    files:
      - AGENTS.md
      - AGENT-LINK.md
      - .scafld/config.local.yaml
      - .priv/secret.md
      - .envrc
      - nested/.env.production
      - ../outside.md
      - ..\\outside.md
      - C:\\secret.md
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("agent contract"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.local.yaml"), []byte("secret: true"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".envrc"), []byte("export TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", ".env.production"), []byte("TOKEN=secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	privateDir := filepath.Join(root, ".priv")
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(privateDir, "secret.md"), []byte("private"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(".priv", "secret.md"), filepath.Join(root, "AGENT-LINK.md")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task", PrintContext: true})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Provider != nil {
		t.Fatalf("print context should not select a provider: %T", selected.Provider)
	}
	if selected.ContextMaxBytes != 4096 || selected.RequiredContextMaxBytes != 65536 {
		t.Fatalf("context budgets = %d/%d", selected.ContextMaxBytes, selected.RequiredContextMaxBytes)
	}
	if selected.Contract.Role != "review" || !strings.Contains(selected.Contract.Body, "senior engineer who gets paged") {
		t.Fatalf("review contract was not loaded: %+v", selected.Contract)
	}
	if len(selected.ContextSections) != 1 || selected.ContextSections[0].Title != "Project Context: AGENTS.md" || !strings.Contains(selected.ContextSections[0].Body, "agent contract") {
		t.Fatalf("context sections = %+v", selected.ContextSections)
	}
}

func TestSelectLoadsClaudeRulesDirectoryAsReviewContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  context:
    files:
      - .claude/rules
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude", "rules", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "rules", "style.md"), []byte("style rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "rules", "nested", "review.mdc"), []byte("review rules"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude", "rules", "image.png"), []byte("not text"), 0o644); err != nil {
		t.Fatal(err)
	}
	selected, err := Select(context.Background(), Options{Root: root, TaskID: "task", PrintContext: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.ContextSections) != 2 {
		t.Fatalf("context sections = %+v", selected.ContextSections)
	}
	got := selected.ContextSections[0].Title + "\n" + selected.ContextSections[1].Title
	for _, want := range []string{"Project Context: .claude/rules/nested/review.mdc", "Project Context: .claude/rules/style.md"} {
		if !strings.Contains(got, want) {
			t.Fatalf("context titles %q missing %q", got, want)
		}
	}
}
