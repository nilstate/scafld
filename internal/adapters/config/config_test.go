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
  absolute_timeout_seconds: 600
  idle_timeout_seconds: 90
  path_prepend:
    - "$HOME/.rbenv/shims"
  env:
    BUNDLE_GEMFILE: "api/Gemfile"
invariants:
  canonical:
    tenant_isolation: "Do not leak data across tenants."
harden:
  max_issues_per_round: 5
  context_max_bytes: 2048
  required_context_max_bytes: 65536
  external:
    provider: "gemini"
    idle_timeout_seconds: 21
    absolute_max_seconds: 43
    gemini:
      model: "gemini-harden"
      binary: "gemini-harden-bin"
      endpoint_host: "generativelanguage.googleapis.com"
review:
  external:
    provider: "codex"
    idle_timeout_seconds: 12
    absolute_max_seconds: 34
    codex:
      model: "gpt-config"
      model_reasoning_effort: "high"
      endpoint_url: "https://api.openai.com"
    claude:
      model: "claude-config"
      effort: "high"
      endpoint_host: "api.anthropic.com"
    gemini:
      model: "gemini-config"
      binary: "gemini-bin"
  context:
    max_bytes: 4096
    required_max_bytes: 65536
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
	if cfg.Harden.MaxIssuesPerRound != 5 {
		t.Fatalf("harden config = %+v", cfg.Harden)
	}
	if cfg.Harden.ContextMaxBytes != 2048 || cfg.Harden.RequiredContextMaxBytes != 65536 || cfg.Harden.External.Provider != "gemini" || cfg.Harden.External.Gemini.Model != "gemini-harden" || cfg.Harden.External.Gemini.Binary != "gemini-harden-bin" || cfg.Harden.External.Gemini.EndpointHost != "generativelanguage.googleapis.com" || cfg.Harden.External.IdleTimeoutSeconds != 21 || cfg.Harden.External.AbsoluteMaxSeconds != 43 {
		t.Fatalf("harden external config = %+v", cfg.Harden.External)
	}
	if len(cfg.Execution.PathPrepend) != 1 || cfg.Execution.PathPrepend[0] != "$HOME/.rbenv/shims" || cfg.Execution.Env["BUNDLE_GEMFILE"] != "api/Gemfile" || cfg.Execution.AbsoluteTimeoutSeconds != 600 || cfg.Execution.IdleTimeoutSeconds != 90 {
		t.Fatalf("execution config = %+v", cfg.Execution)
	}
	if cfg.Invariants.Canonical["tenant_isolation"] != "Do not leak data across tenants." {
		t.Fatalf("invariants = %+v", cfg.Invariants.Canonical)
	}
	if cfg.Review.External.Provider != "codex" || cfg.Review.External.Codex.Model != "gpt-config" || cfg.Review.External.Codex.ModelReasoningEffort != "high" || cfg.Review.External.Codex.EndpointURL != "https://api.openai.com" || cfg.Review.External.Claude.Model != "claude-config" || cfg.Review.External.Claude.Effort != "high" || cfg.Review.External.Claude.EndpointHost != "api.anthropic.com" || cfg.Review.External.Gemini.Model != "gemini-config" || cfg.Review.External.Gemini.Binary != "gemini-bin" {
		t.Fatalf("review config = %+v", cfg.Review.External)
	}
	if cfg.Review.External.IdleTimeoutSeconds != 12 || cfg.Review.External.AbsoluteMaxSeconds != 34 {
		t.Fatalf("timeouts = %+v", cfg.Review.External)
	}
	if cfg.Review.Context.MaxBytes != 4096 || cfg.Review.Context.RequiredMaxBytes != 65536 || !contains(cfg.Review.Context.Files, "AGENTS.md") {
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
      model_reasoning_effort: "medium"
      endpoint_host: "api.openai.com"
    claude:
      model: "claude-config"
      effort: "medium"
harden:
  external:
    provider: "codex"
    codex:
      model: "gpt-harden-config"
      model_reasoning_effort: "high"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.local.yaml"), []byte(`
execution:
  absolute_timeout_seconds: 900
  path_prepend:
    - "$HOME/.local/bin"
  env:
    RUBYOPT: "-W0"
review:
  external:
    provider: "claude"
    codex:
      model_reasoning_effort: "xhigh"
    claude:
      model: "claude-local"
      effort: "xhigh"
      endpoint_host: "api.anthropic.com"
  context:
    files:
      - MEMORY.md
  adversarial_passes:
    regression_hunt:
      description: "Local override"
harden:
  external:
    provider: "gemini"
    gemini:
      model: "gemini-harden-local"
      binary: "gemini-local-bin"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Review.External.Provider != "claude" || cfg.Review.External.Claude.Model != "claude-local" || cfg.Review.External.Claude.EndpointHost != "api.anthropic.com" {
		t.Fatalf("local overlay did not apply: %+v", cfg.Review.External)
	}
	if cfg.Review.External.Claude.Effort != "xhigh" {
		t.Fatalf("local claude effort did not apply: %+v", cfg.Review.External.Claude)
	}
	if cfg.Review.External.Codex.Model != "gpt-config" || cfg.Review.External.Codex.ModelReasoningEffort != "xhigh" || cfg.Review.External.Codex.EndpointHost != "api.openai.com" || cfg.Review.External.AbsoluteMaxSeconds != 34 {
		t.Fatalf("base values were not preserved: %+v", cfg.Review.External)
	}
	if cfg.Harden.MaxIssuesPerRound != 8 || cfg.Harden.External.Provider != "gemini" || cfg.Harden.External.Codex.Model != "gpt-harden-config" || cfg.Harden.External.Codex.ModelReasoningEffort != "high" || cfg.Harden.External.Gemini.Model != "gemini-harden-local" || cfg.Harden.External.Gemini.Binary != "gemini-local-bin" {
		t.Fatalf("harden overlay/defaults not applied: %+v", cfg.Harden)
	}
	if !contains(cfg.Review.Context.Files, "MEMORY.md") || !contains(cfg.Review.Context.Files, "AGENTS.md") {
		t.Fatalf("review context overlay did not apply: %+v", cfg.Review.Context)
	}
	if len(cfg.Execution.PathPrepend) != 1 || cfg.Execution.PathPrepend[0] != "$HOME/.local/bin" || cfg.Execution.Env["RUBYOPT"] != "-W0" || cfg.Execution.AbsoluteTimeoutSeconds != 900 {
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

func TestDetectExecutionFindsVersionManagerShims(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "api/.ruby-version", "3.4.5\n")
	writeFile(t, root, "web/.node-version", "24.0.0\n")
	writeFile(t, root, "worker/.python-version", "3.13.0\n")
	writeFile(t, root, ".tool-versions", "nodejs 24.0.0\npython 3.13.0\n")
	writeFile(t, root, "service/mise.toml", "[tools]\ngo = '1.25.0'\n")

	detected := DetectExecution(root)
	for _, want := range []string{"$HOME/.rbenv/shims", "$HOME/.nodenv/shims", "$HOME/.pyenv/shims", "$HOME/.asdf/shims", "$HOME/.local/share/mise/shims", "$HOME/.mise/shims"} {
		if !contains(detected.Execution.PathPrepend, want) {
			t.Fatalf("path_prepend = %+v, missing %s", detected.Execution.PathPrepend, want)
		}
	}
	for _, want := range []string{"api/.ruby-version", "web/.node-version", "worker/.python-version", ".tool-versions", "service/mise.toml"} {
		if !contains(detected.Sources, want) {
			t.Fatalf("sources = %+v, missing %s", detected.Sources, want)
		}
	}
}

func TestEffectiveExecutionPutsExplicitConfigBeforeDetectedShims(t *testing.T) {
	t.Setenv("HOME", "/tmp/scafld-home")
	t.Setenv("PATH", "/usr/bin")

	root := t.TempDir()
	writeFile(t, root, "api/.ruby-version", "3.4.5\n")

	env := EffectiveExecution(root, ExecutionConfig{
		PathPrepend:            []string{"$HOME/custom-shims"},
		Env:                    map[string]string{"BUNDLE_GEMFILE": "api/Gemfile"},
		AbsoluteTimeoutSeconds: 600,
		IdleTimeoutSeconds:     60,
	}).ProcessEnv()
	joined := strings.Join(env, "\n")
	wantPath := "PATH=/tmp/scafld-home/custom-shims" +
		string(os.PathListSeparator) + "/tmp/scafld-home/.rbenv/shims" +
		string(os.PathListSeparator) + "/usr/bin"
	if !strings.Contains(joined, wantPath) {
		t.Fatalf("PATH override missing %q in %+v", wantPath, env)
	}
	if !strings.Contains(joined, "BUNDLE_GEMFILE=api/Gemfile") {
		t.Fatalf("explicit env missing in %+v", env)
	}
	effective := EffectiveExecution(root, ExecutionConfig{AbsoluteTimeoutSeconds: 600, IdleTimeoutSeconds: 60})
	if effective.AbsoluteTimeoutSeconds != 600 || effective.IdleTimeoutSeconds != 60 {
		t.Fatalf("execution timeouts not preserved: %+v", effective)
	}
}

func TestConfigDefaultWhenMissing(t *testing.T) {
	t.Parallel()
	cfg, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Review.External.Provider != "auto" || cfg.Review.External.Codex.Model != DefaultCodexModel || cfg.Review.External.Claude.Model != DefaultClaudeModel {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.Review.External.Codex.ModelReasoningEffort != DefaultCodexModelReasoningEffort || cfg.Harden.External.Codex.ModelReasoningEffort != DefaultCodexModelReasoningEffort {
		t.Fatalf("codex reasoning defaults missing: review=%q harden=%q", cfg.Review.External.Codex.ModelReasoningEffort, cfg.Harden.External.Codex.ModelReasoningEffort)
	}
	if cfg.Review.External.Claude.Effort != DefaultClaudeEffort || cfg.Harden.External.Claude.Effort != DefaultClaudeEffort {
		t.Fatalf("claude effort defaults missing: review=%q harden=%q", cfg.Review.External.Claude.Effort, cfg.Harden.External.Claude.Effort)
	}
	if cfg.Harden.External.Codex.Model != DefaultCodexModel || cfg.Harden.External.Claude.Model != DefaultClaudeModel {
		t.Fatalf("harden defaults = %+v", cfg.Harden.External)
	}
	if len(cfg.Review.AdversarialPasses) == 0 || len(cfg.Review.AutomatedPasses) == 0 {
		t.Fatalf("default review passes missing = %+v", cfg.Review)
	}
	if cfg.Harden.MaxIssuesPerRound != 8 {
		t.Fatalf("default harden config missing = %+v", cfg.Harden)
	}
	if cfg.Harden.ContextMaxBytes != 16384 || cfg.Harden.RequiredContextMaxBytes != 131072 || cfg.Review.Context.MaxBytes != 16384 || cfg.Review.Context.RequiredMaxBytes != 131072 {
		t.Fatalf("default context budgets missing = harden:%+v review:%+v", cfg.Harden, cfg.Review.Context)
	}
	if cfg.Execution.AbsoluteTimeoutSeconds != 300 {
		t.Fatalf("default execution timeout missing = %+v", cfg.Execution)
	}
	if cfg.Review.Dossier.MaxFindings <= 0 || cfg.Review.Dossier.MinAttackAngles <= 0 || cfg.Review.Dossier.ReviewDepth == "" || cfg.Review.Dossier.RerunPolicy == "" {
		t.Fatalf("default review dossier config missing = %+v", cfg.Review.Dossier)
	}
}

func TestProviderModelsUpgradeToLatestAliasesWhenPossible(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, ".scafld/config.yaml", `
harden:
  external:
    codex:
      model: "gpt-5.5"
    claude:
      model: "claude-opus-4-7"
      effort: "default"
review:
  external:
    codex:
      model: "gpt-5-codex"
    claude:
      model: "claude-3-5-sonnet-20241022"
      effort: "XHIGH"
`)

	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Harden.External.Codex.Model != DefaultCodexModel || cfg.Harden.External.Claude.Model != DefaultClaudeModel {
		t.Fatalf("harden provider models were not upgraded: %+v", cfg.Harden.External)
	}
	if cfg.Review.External.Codex.Model != DefaultCodexModel || cfg.Review.External.Claude.Model != "sonnet" {
		t.Fatalf("review provider models were not upgraded: %+v", cfg.Review.External)
	}
	if cfg.Harden.External.Claude.Effort != DefaultClaudeEffort || cfg.Review.External.Claude.Effort != "xhigh" {
		t.Fatalf("claude efforts were not normalized: harden=%q review=%q", cfg.Harden.External.Claude.Effort, cfg.Review.External.Claude.Effort)
	}

	base, err := LoadBase(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if base.Review.External.Codex.Model != DefaultCodexModel || base.Review.External.Claude.Model != "sonnet" {
		t.Fatalf("base provider models were not upgraded: %+v", base.Review.External)
	}
}

func TestCodexLatestAliasResolvesToDefaultModel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, ".scafld/config.yaml", `
harden:
  external:
    codex:
      model: "latest"
      model_reasoning_effort: "default"
review:
  external:
    codex:
      model: "current"
      model_reasoning_effort: "XHIGH"
`)

	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Harden.External.Codex.Model != DefaultCodexModel || cfg.Review.External.Codex.Model != DefaultCodexModel {
		t.Fatalf("codex latest aliases were not resolved: harden=%q review=%q default=%q", cfg.Harden.External.Codex.Model, cfg.Review.External.Codex.Model, DefaultCodexModel)
	}
	if cfg.Harden.External.Codex.ModelReasoningEffort != DefaultCodexModelReasoningEffort || cfg.Review.External.Codex.ModelReasoningEffort != "xhigh" {
		t.Fatalf("codex reasoning efforts were not normalized: harden=%q review=%q", cfg.Harden.External.Codex.ModelReasoningEffort, cfg.Review.External.Codex.ModelReasoningEffort)
	}
}

func TestClaudeConcreteModelsNormalizeToLatestAliases(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"claude-opus-4-7":            "opus",
		"claude-opus-4.8":            "opus",
		"opus-4.7":                   "opus",
		"claude-3-5-sonnet-20241022": "sonnet",
		"claude-haiku-4-5-20251001":  "haiku",
		" OPUS ":                     "opus",
		"claude-config":              "claude-config",
		"custom-sonnet-model":        "custom-sonnet-model",
	}
	for model, want := range tests {
		if got := normalizeClaudeLatestModel(model); got != want {
			t.Fatalf("normalizeClaudeLatestModel(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestVerifyPolicyDefaultsLocalAndOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Verify.Policy != "local" {
		t.Fatalf("default verify.policy = %q, want local", cfg.Verify.Policy)
	}

	for _, want := range []string{"advisory", "required", "local"} {
		root := t.TempDir()
		writeFile(t, root, ".scafld/config.yaml", "verify:\n  policy: "+want+"\n")
		cfg, err := Load(context.Background(), root)
		if err != nil {
			t.Fatalf("load policy %q: %v", want, err)
		}
		if cfg.Verify.Policy != want {
			t.Fatalf("verify.policy = %q, want %q", cfg.Verify.Policy, want)
		}
	}
}

func TestVerifyPolicyRejectsUnknownValue(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, ".scafld/config.yaml", "verify:\n  policy: blocking\n")
	if _, err := Load(context.Background(), root); err == nil {
		t.Fatal("Load accepted invalid verify.policy, want error")
	}
	// The verify accountability path (LoadBase) rejects it too.
	if _, err := LoadBase(context.Background(), root); err == nil {
		t.Fatal("LoadBase accepted invalid verify.policy, want error")
	}
}

func writeFile(t *testing.T, root string, rel string, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestConfigRejectsInvariantList(t *testing.T) {
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
	if strings.Contains(text, "scafld update") {
		t.Fatalf("error %q should not advertise an update-time repair", text)
	}
	for _, want := range []string{"invariants.canonical must be a mapping", "not a list"} {
		if !strings.Contains(text, want) {
			t.Fatalf("error %q missing %q", text, want)
		}
	}
}
