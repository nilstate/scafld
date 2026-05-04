package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
version: "1.0"
llm:
  model_profile: "team-default"
harden:
  max_questions_per_round: 5
review:
  external:
    provider: "codex"
    idle_timeout_seconds: 12
    absolute_max_seconds: 34
    codex:
      model: "gpt-config"
    claude:
      model: "claude-config"
  automated_passes:
    spec_compliance:
      order: 10
      title: "Spec Compliance"
      description: "Verify spec evidence"
  adversarial_passes:
    regression_hunt:
      order: 30
      title: "Regression Hunt"
      description: "Trace downstream consumers"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "1.0" || cfg.LLM.ModelProfile != "team-default" {
		t.Fatalf("config = %+v", cfg)
	}
	if cfg.Harden.MaxQuestionsPerRound != 5 {
		t.Fatalf("harden config = %+v", cfg.Harden)
	}
	if cfg.Review.External.Provider != "codex" || cfg.Review.External.Codex.Model != "gpt-config" || cfg.Review.External.Claude.Model != "claude-config" {
		t.Fatalf("review config = %+v", cfg.Review.External)
	}
	if cfg.Review.External.IdleTimeoutSeconds != 12 || cfg.Review.External.AbsoluteMaxSeconds != 34 {
		t.Fatalf("timeouts = %+v", cfg.Review.External)
	}
	if cfg.Review.AutomatedPasses["spec_compliance"].Title != "Spec Compliance" || cfg.Review.AdversarialPasses["regression_hunt"].Order != 30 {
		t.Fatalf("review passes = %+v %+v", cfg.Review.AutomatedPasses, cfg.Review.AdversarialPasses)
	}
}

func TestConfigLocalOverlay(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
review:
  external:
    provider: "codex"
    idle_timeout_seconds: 12
    absolute_max_seconds: 34
    codex:
      model: "gpt-config"
    claude:
      model: "claude-config"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.local.yaml"), []byte(`
review:
  external:
    provider: "claude"
    claude:
      model: "claude-local"
  adversarial_passes:
    regression_hunt:
      description: "Local override"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Review.External.Provider != "claude" || cfg.Review.External.Claude.Model != "claude-local" {
		t.Fatalf("local overlay did not apply: %+v", cfg.Review.External)
	}
	if cfg.Review.External.Codex.Model != "gpt-config" || cfg.Review.External.AbsoluteMaxSeconds != 34 {
		t.Fatalf("base values were not preserved: %+v", cfg.Review.External)
	}
	if cfg.Harden.MaxQuestionsPerRound != 8 {
		t.Fatalf("default harden config not applied: %+v", cfg.Harden)
	}
	if cfg.Review.AdversarialPasses["regression_hunt"].Description != "Local override" {
		t.Fatalf("review pass overlay did not apply: %+v", cfg.Review.AdversarialPasses)
	}
}

func TestConfigDefaultWhenMissing(t *testing.T) {
	t.Parallel()
	cfg, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Review.External.Provider != "auto" || cfg.Review.External.Codex.Model == "" || cfg.Review.External.Claude.Model == "" {
		t.Fatalf("defaults = %+v", cfg)
	}
	if len(cfg.Review.AdversarialPasses) == 0 || len(cfg.Review.AutomatedPasses) == 0 {
		t.Fatalf("default review passes missing = %+v", cfg.Review)
	}
	if cfg.Harden.MaxQuestionsPerRound != 8 {
		t.Fatalf("default harden config missing = %+v", cfg.Harden)
	}
}
