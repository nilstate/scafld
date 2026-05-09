package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
execution:
  path_prepend:
    - "$HOME/.rbenv/shims"
  env:
    BUNDLE_GEMFILE: "api/Gemfile"
invariants:
  canonical:
    tenant_isolation: "Do not leak data across tenants."
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
  context:
    max_bytes: 4096
    files:
      - AGENTS.md
  dossier:
    max_findings: 9
    min_attack_angles: 4
    review_depth: "deep"
    rerun_policy: "verify_open_blockers"
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
	if len(cfg.Execution.PathPrepend) != 1 || cfg.Execution.PathPrepend[0] != "$HOME/.rbenv/shims" || cfg.Execution.Env["BUNDLE_GEMFILE"] != "api/Gemfile" {
		t.Fatalf("execution config = %+v", cfg.Execution)
	}
	if cfg.Invariants.Canonical["tenant_isolation"] != "Do not leak data across tenants." {
		t.Fatalf("invariants = %+v", cfg.Invariants.Canonical)
	}
	if cfg.Review.External.Provider != "codex" || cfg.Review.External.Codex.Model != "gpt-config" || cfg.Review.External.Claude.Model != "claude-config" {
		t.Fatalf("review config = %+v", cfg.Review.External)
	}
	if cfg.Review.External.IdleTimeoutSeconds != 12 || cfg.Review.External.AbsoluteMaxSeconds != 34 {
		t.Fatalf("timeouts = %+v", cfg.Review.External)
	}
	if cfg.Review.Context.MaxBytes != 4096 || !contains(cfg.Review.Context.Files, "AGENTS.md") {
		t.Fatalf("review context = %+v", cfg.Review.Context)
	}
	if cfg.Review.Dossier.MaxFindings != 9 || cfg.Review.Dossier.MinAttackAngles != 4 || cfg.Review.Dossier.ReviewDepth != "deep" || cfg.Review.Dossier.RerunPolicy != "verify_open_blockers" {
		t.Fatalf("review dossier config = %+v", cfg.Review.Dossier)
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
execution:
  path_prepend:
    - "$HOME/.local/bin"
  env:
    RUBYOPT: "-W0"
review:
  external:
    provider: "claude"
    claude:
      model: "claude-local"
  context:
    files:
      - MEMORY.md
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
	if !contains(cfg.Review.Context.Files, "MEMORY.md") || !contains(cfg.Review.Context.Files, "AGENTS.md") {
		t.Fatalf("review context overlay did not apply: %+v", cfg.Review.Context)
	}
	if len(cfg.Execution.PathPrepend) != 1 || cfg.Execution.PathPrepend[0] != "$HOME/.local/bin" || cfg.Execution.Env["RUBYOPT"] != "-W0" {
		t.Fatalf("execution overlay did not apply: %+v", cfg.Execution)
	}
	if cfg.Review.AdversarialPasses["regression_hunt"].Description != "Local override" {
		t.Fatalf("review pass overlay did not apply: %+v", cfg.Review.AdversarialPasses)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestExecutionEnvExpandsPathPrependAndEnv(t *testing.T) {
	t.Setenv("HOME", "/tmp/scafld-home")
	t.Setenv("PATH", "/usr/bin")

	env := (ExecutionConfig{
		PathPrepend: []string{"$HOME/.rbenv/shims", "~/.rbenv/bin"},
		Env:         map[string]string{"BUNDLE_GEMFILE": "$HOME/app/Gemfile"},
	}).ProcessEnv()
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "BUNDLE_GEMFILE=/tmp/scafld-home/app/Gemfile") {
		t.Fatalf("env did not expand env vars: %+v", env)
	}
	wantPath := "PATH=/tmp/scafld-home/.rbenv/shims" + string(os.PathListSeparator) + "/tmp/scafld-home/.rbenv/bin" + string(os.PathListSeparator) + "/usr/bin"
	if !strings.Contains(joined, wantPath) {
		t.Fatalf("PATH override missing %q in %+v", wantPath, env)
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
	if cfg.Review.Dossier.MaxFindings <= 0 || cfg.Review.Dossier.MinAttackAngles <= 0 || cfg.Review.Dossier.ReviewDepth == "" || cfg.Review.Dossier.RerunPolicy == "" {
		t.Fatalf("default review dossier config missing = %+v", cfg.Review.Dossier)
	}
}

func TestConfigRejectsInvariantListWithUpdateHint(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte(`
version: "1.0"
invariants:
  canonical:
    - domain_boundaries
    - no_legacy_code
`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(context.Background(), root)
	if err == nil {
		t.Fatal("Load succeeded, want strict config shape error")
	}
	text := err.Error()
	for _, want := range []string{"invariants.canonical must be a mapping", "scafld update"} {
		if !strings.Contains(text, want) {
			t.Fatalf("error %q missing %q", text, want)
		}
	}
}
