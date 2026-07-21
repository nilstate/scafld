package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultCodexModel is empty by design: scafld omits -m for the "latest"
	// Codex model path so Codex CLI can use its own current default.
	DefaultCodexModel = ""
	// DefaultCodexModelReasoningEffort keeps governed review/harden runs from
	// inheriting a medium-effort CLI default after user config is isolated.
	DefaultCodexModelReasoningEffort = "xhigh"
	// DefaultClaudeModel uses Claude Code's rolling latest-Opus alias.
	DefaultClaudeModel = "opus"
	// DefaultClaudeEffort uses Claude Code's native effort control for governed
	// review/harden runs.
	DefaultClaudeEffort = "xhigh"
)

var legacyCodexDefaultModels = map[string]string{
	"latest":        DefaultCodexModel,
	"current":       DefaultCodexModel,
	"default":       DefaultCodexModel,
	"gpt-5.5":       DefaultCodexModel,
	"gpt-5-codex":   DefaultCodexModel,
	"gpt-5.2-codex": DefaultCodexModel,
}

var claudeLatestAliases = map[string]string{
	"opus":   "opus",
	"sonnet": "sonnet",
	"haiku":  "haiku",
}

var claudeLatestFamilies = []string{"opus", "sonnet", "haiku"}

// Config is the merged runtime configuration for a scafld workspace.
type Config struct {
	Version    string          `yaml:"version"`
	Invariants InvariantConfig `yaml:"invariants"`
	LLM        LLMConfig       `yaml:"llm"`
	Execution  ExecutionConfig `yaml:"execution"`
	Verify     VerifyConfig    `yaml:"verify"`
	Harden     HardenConfig    `yaml:"harden"`
	Review     ReviewConfig    `yaml:"review"`
}

// InvariantConfig names project-level invariant IDs available to specs.
type InvariantConfig struct {
	Canonical map[string]string `yaml:"canonical"`
}

// LLMConfig contains shared model-profile settings.
type LLMConfig struct {
	ModelProfile string `yaml:"model_profile"`
}

// ExecutionConfig controls the deterministic environment for acceptance commands.
type ExecutionConfig struct {
	PathPrepend            []string          `yaml:"path_prepend"`
	Env                    map[string]string `yaml:"env"`
	AbsoluteTimeoutSeconds int               `yaml:"absolute_timeout_seconds"`
	IdleTimeoutSeconds     int               `yaml:"idle_timeout_seconds"`
}

// VerifyConfig controls CI receipt verification defaults.
type VerifyConfig struct {
	MinIndependence string `yaml:"min_independence"`
	TrustedKeysPath string `yaml:"trusted_keys_path"`
	ReceiptPath     string `yaml:"receipt_path"`
	// Policy declares the operator's intended enforcement tier (local, advisory,
	// required). It is reporting-only metadata: it is read by `scafld verify
	// --self-check` and the docs framing, never by the receipt-verification
	// verdict path, so a tier can never bypass a real check.
	Policy string `yaml:"policy"`
}

// HardenConfig controls hardening prompt behavior.
type HardenConfig struct {
	MaxIssuesPerRound       int                  `yaml:"max_issues_per_round"`
	ContextMaxBytes         int                  `yaml:"context_max_bytes"`
	RequiredContextMaxBytes int                  `yaml:"required_context_max_bytes"`
	External                ExternalReviewConfig `yaml:"external"`
}

// ReviewConfig controls automated and adversarial review behavior.
type ReviewConfig struct {
	External          ExternalReviewConfig        `yaml:"external"`
	Context           ReviewContextConfig         `yaml:"context"`
	Dossier           ReviewDossierConfig         `yaml:"dossier"`
	AutomatedPasses   map[string]ReviewPassConfig `yaml:"automated_passes"`
	AdversarialPasses map[string]ReviewPassConfig `yaml:"adversarial_passes"`
}

// ReviewDossierConfig controls default review dossier budget and rerun behavior.
type ReviewDossierConfig struct {
	MaxFindings     int    `yaml:"max_findings"`
	MinAttackAngles int    `yaml:"min_attack_angles"`
	ReviewDepth     string `yaml:"review_depth"`
	RerunPolicy     string `yaml:"rerun_policy"`
}

// ReviewContextConfig controls bounded project context sent to reviewers.
type ReviewContextConfig struct {
	MaxBytes         int      `yaml:"max_bytes"`
	RequiredMaxBytes int      `yaml:"required_max_bytes"`
	Files            []string `yaml:"files"`
}

// ExternalReviewConfig configures external model-provider review execution.
type ExternalReviewConfig struct {
	Provider           string               `yaml:"provider"`
	Command            string               `yaml:"command"`
	ProviderBinary     string               `yaml:"provider_binary"`
	IdleTimeoutSeconds int                  `yaml:"idle_timeout_seconds"`
	AbsoluteMaxSeconds int                  `yaml:"absolute_max_seconds"`
	FallbackPolicy     string               `yaml:"fallback_policy"`
	Codex              CodexProviderConfig  `yaml:"codex"`
	Claude             ClaudeProviderConfig `yaml:"claude"`
	Gemini             ProviderConfig       `yaml:"gemini"`
}

// CodexProviderConfig configures the Codex CLI adapter. ModelReasoningEffort is
// intentionally Codex-specific; other provider thinking controls need their own
// verified adapter fields before scafld exposes them.
type CodexProviderConfig struct {
	ProviderConfig `yaml:",inline"`

	ModelReasoningEffort string `yaml:"model_reasoning_effort"`
}

// ClaudeProviderConfig configures the Claude Code adapter.
type ClaudeProviderConfig struct {
	ProviderConfig `yaml:",inline"`

	Effort string `yaml:"effort"`
}

// ProviderConfig configures a named external provider implementation.
type ProviderConfig struct {
	Model        string `yaml:"model"`
	Binary       string `yaml:"binary"`
	EndpointURL  string `yaml:"endpoint_url"`
	EndpointHost string `yaml:"endpoint_host"`
}

// ReviewPassConfig describes one review pass in the review agenda.
type ReviewPassConfig struct {
	Order       int    `yaml:"order"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
}

// Load reads base config and local overrides from root.
func Load(ctx context.Context, root string) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	cfg, err := readConfigFile(filepath.Join(root, ".scafld", "config.yaml"), false)
	if err != nil {
		return Config{}, err
	}
	local, err := readConfigFile(filepath.Join(root, ".scafld", "config.local.yaml"), true)
	if err != nil {
		return Config{}, err
	}
	effective := overlay(cfg, local)
	if err := validateVerifyConfig(effective.Verify); err != nil {
		return Config{}, err
	}
	return effective, nil
}

// LoadBase reads only the committed base config, ignoring config.local overrides.
// The gate and verify paths use this so a host-committed config.local.yaml cannot
// repoint the trust anchor, relax the independence policy, or redirect reviewer
// endpoints. config.local stays an authoring convenience for the human lifecycle,
// never an input to the accountability gate.
func LoadBase(ctx context.Context, root string) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	cfg, err := readConfigFile(filepath.Join(root, ".scafld", "config.yaml"), false)
	if err != nil {
		return Config{}, err
	}
	effective := overlay(cfg, Config{})
	if err := validateVerifyConfig(effective.Verify); err != nil {
		return Config{}, err
	}
	return effective, nil
}

func readConfigFile(path string, optional bool) (Config, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if optional && errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	if len(data) == 0 {
		return Config{}, nil
	}
	if err := validateConfigShape(path, data); err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

func validateConfigShape(path string, data []byte) error {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	root := documentRoot(&doc)
	canonical := mappingLookup(mappingLookup(root, "invariants"), "canonical")
	if canonical == nil {
		return nil
	}
	if canonical.Kind == yaml.SequenceNode {
		return fmt.Errorf("parse config %s: invariants.canonical must be a mapping of id to description, not a list", path)
	}
	if canonical.Kind != yaml.MappingNode {
		return fmt.Errorf("parse config %s: invariants.canonical must be a mapping of id to description", path)
	}
	return nil
}

func documentRoot(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func mappingLookup(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// Default returns the built-in workspace configuration.
func Default() Config {
	return Config{
		Version: "1.0",
		Invariants: InvariantConfig{Canonical: map[string]string{
			"domain_boundaries":           "Respect layer separation and ownership boundaries.",
			"no_legacy_code":              "Do not add dual-reads, dual-writes, runtime fallbacks, or compatibility shims.",
			"no_test_logic_in_production": "Keep fixtures, mocks, and test-only branches out of production paths.",
			"public_api_stable":           "Do not change public schemas, migrations, HTTP contracts, or event shapes without explicit approval.",
			"config_from_env":             "Keep secrets and environment-specific configuration out of source code.",
		}},
		LLM: LLMConfig{ModelProfile: "default"},
		Execution: ExecutionConfig{
			AbsoluteTimeoutSeconds: 300,
		},
		Verify: VerifyConfig{
			MinIndependence: "isolation_only",
			TrustedKeysPath: ".scafld/trusted-keys.json",
			ReceiptPath:     ".scafld/receipts/latest.json",
			Policy:          "local",
		},
		Harden: HardenConfig{
			MaxIssuesPerRound:       8,
			ContextMaxBytes:         16384,
			RequiredContextMaxBytes: 131072,
			External: ExternalReviewConfig{
				IdleTimeoutSeconds: 180,
				AbsoluteMaxSeconds: 1800,
				FallbackPolicy:     "disable",
				Codex: CodexProviderConfig{
					ProviderConfig:       ProviderConfig{Model: DefaultCodexModel},
					ModelReasoningEffort: DefaultCodexModelReasoningEffort,
				},
				Claude: ClaudeProviderConfig{
					ProviderConfig: ProviderConfig{Model: DefaultClaudeModel},
					Effort:         DefaultClaudeEffort,
				},
				Gemini: ProviderConfig{},
			},
		},
		Review: ReviewConfig{
			External: ExternalReviewConfig{
				Provider:           "auto",
				IdleTimeoutSeconds: 180,
				AbsoluteMaxSeconds: 1800,
				FallbackPolicy:     "disable",
				Codex: CodexProviderConfig{
					ProviderConfig:       ProviderConfig{Model: DefaultCodexModel},
					ModelReasoningEffort: DefaultCodexModelReasoningEffort,
				},
				Claude: ClaudeProviderConfig{
					ProviderConfig: ProviderConfig{Model: DefaultClaudeModel},
					Effort:         DefaultClaudeEffort,
				},
				Gemini: ProviderConfig{},
			},
			Context: ReviewContextConfig{
				MaxBytes:         16384,
				RequiredMaxBytes: 131072,
				Files: []string{
					"AGENTS.md",
					"CLAUDE.md",
					".claude/rules",
					"README.md",
					"docs/review.md",
					"docs/configuration.md",
					"docs/execution.md",
					".scafld/core/schemas/review_dossier.json",
				},
			},
			Dossier: ReviewDossierConfig{
				MaxFindings:     12,
				MinAttackAngles: 6,
				ReviewDepth:     "standard",
				RerunPolicy:     "verify_open_blockers",
			},
			AutomatedPasses: map[string]ReviewPassConfig{
				"spec_compliance": {Order: 10, Title: "Spec Compliance", Description: "Verify recorded acceptance evidence against the spec"},
				"ambient_drift":   {Order: 20, Title: "Ambient Drift", Description: "Compare task scope against current workspace changes and separate task changes from unrelated workspace drift"},
			},
			AdversarialPasses: map[string]ReviewPassConfig{
				"regression_hunt":  {Order: 30, Title: "Regression Hunt", Description: "Trace callers, importers, and downstream consumers for regressions"},
				"convention_check": {Order: 40, Title: "Convention Check", Description: "Check changed code against declared invariants, spec constraints, and root agent guidance"},
				"dark_patterns":    {Order: 50, Title: "Dark Patterns", Description: "Hunt for subtle bugs, hardcodes, races, and safety gaps"},
			},
		},
	}
}

func overlay(base Config, local Config) Config {
	if local.Version != "" {
		base.Version = local.Version
	}
	base.Invariants.Canonical = overlayStrings(base.Invariants.Canonical, local.Invariants.Canonical)
	if local.LLM.ModelProfile != "" {
		base.LLM.ModelProfile = local.LLM.ModelProfile
	}
	base.Execution = overlayExecution(base.Execution, local.Execution)
	base.Verify = overlayVerify(base.Verify, local.Verify)
	if local.Harden.MaxIssuesPerRound > 0 {
		base.Harden.MaxIssuesPerRound = local.Harden.MaxIssuesPerRound
	}
	if local.Harden.ContextMaxBytes > 0 {
		base.Harden.ContextMaxBytes = local.Harden.ContextMaxBytes
	}
	if local.Harden.RequiredContextMaxBytes > 0 {
		base.Harden.RequiredContextMaxBytes = local.Harden.RequiredContextMaxBytes
	}
	base.Harden.External = overlayExternal(base.Harden.External, local.Harden.External)
	base.Review.External = overlayExternal(base.Review.External, local.Review.External)
	base.Review.Context = overlayReviewContext(base.Review.Context, local.Review.Context)
	base.Review.Dossier = overlayReviewDossier(base.Review.Dossier, local.Review.Dossier)
	base.Review.AutomatedPasses = overlayPasses(base.Review.AutomatedPasses, local.Review.AutomatedPasses)
	base.Review.AdversarialPasses = overlayPasses(base.Review.AdversarialPasses, local.Review.AdversarialPasses)
	return withDefaults(base)
}

func overlayExternal(base ExternalReviewConfig, local ExternalReviewConfig) ExternalReviewConfig {
	if local.Provider != "" {
		base.Provider = local.Provider
	}
	if local.Command != "" {
		base.Command = local.Command
	}
	if local.ProviderBinary != "" {
		base.ProviderBinary = local.ProviderBinary
	}
	if local.IdleTimeoutSeconds > 0 {
		base.IdleTimeoutSeconds = local.IdleTimeoutSeconds
	}
	if local.AbsoluteMaxSeconds > 0 {
		base.AbsoluteMaxSeconds = local.AbsoluteMaxSeconds
	}
	if local.FallbackPolicy != "" {
		base.FallbackPolicy = local.FallbackPolicy
	}
	base.Codex = overlayCodexProvider(base.Codex, local.Codex)
	base.Claude = overlayClaudeProvider(base.Claude, local.Claude)
	base.Gemini = overlayProvider(base.Gemini, local.Gemini)
	return base
}

func overlayCodexProvider(base CodexProviderConfig, local CodexProviderConfig) CodexProviderConfig {
	base.ProviderConfig = overlayProvider(base.ProviderConfig, local.ProviderConfig)
	if local.ModelReasoningEffort != "" {
		base.ModelReasoningEffort = local.ModelReasoningEffort
	}
	return base
}

func overlayClaudeProvider(base ClaudeProviderConfig, local ClaudeProviderConfig) ClaudeProviderConfig {
	base.ProviderConfig = overlayProvider(base.ProviderConfig, local.ProviderConfig)
	if local.Effort != "" {
		base.Effort = local.Effort
	}
	return base
}

func overlayProvider(base ProviderConfig, local ProviderConfig) ProviderConfig {
	if local.Model != "" {
		base.Model = local.Model
	}
	if local.Binary != "" {
		base.Binary = local.Binary
	}
	if local.EndpointURL != "" {
		base.EndpointURL = local.EndpointURL
	}
	if local.EndpointHost != "" {
		base.EndpointHost = local.EndpointHost
	}
	return base
}

func overlayExecution(base ExecutionConfig, local ExecutionConfig) ExecutionConfig {
	if len(local.PathPrepend) > 0 {
		base.PathPrepend = append(append([]string(nil), base.PathPrepend...), local.PathPrepend...)
	}
	base.Env = overlayStrings(base.Env, local.Env)
	if local.AbsoluteTimeoutSeconds > 0 {
		base.AbsoluteTimeoutSeconds = local.AbsoluteTimeoutSeconds
	}
	if local.IdleTimeoutSeconds > 0 {
		base.IdleTimeoutSeconds = local.IdleTimeoutSeconds
	}
	return base
}

func overlayVerify(base VerifyConfig, local VerifyConfig) VerifyConfig {
	if local.MinIndependence != "" {
		base.MinIndependence = local.MinIndependence
	}
	if local.TrustedKeysPath != "" {
		base.TrustedKeysPath = local.TrustedKeysPath
	}
	if local.ReceiptPath != "" {
		base.ReceiptPath = local.ReceiptPath
	}
	if local.Policy != "" {
		base.Policy = local.Policy
	}
	return base
}

// ValidVerifyPolicy reports whether policy is a supported verify enforcement tier.
func ValidVerifyPolicy(policy string) bool {
	switch policy {
	case "local", "advisory", "required":
		return true
	default:
		return false
	}
}

func validateVerifyConfig(v VerifyConfig) error {
	if !ValidVerifyPolicy(v.Policy) {
		return fmt.Errorf("invalid verify.policy %q: want local, advisory, or required", v.Policy)
	}
	return nil
}

func overlayReviewContext(base ReviewContextConfig, local ReviewContextConfig) ReviewContextConfig {
	if local.MaxBytes > 0 {
		base.MaxBytes = local.MaxBytes
	}
	if local.RequiredMaxBytes > 0 {
		base.RequiredMaxBytes = local.RequiredMaxBytes
	}
	if len(local.Files) > 0 {
		base.Files = dedupeList(append(append([]string(nil), base.Files...), local.Files...))
	}
	return base
}

func overlayReviewDossier(base ReviewDossierConfig, local ReviewDossierConfig) ReviewDossierConfig {
	if local.MaxFindings > 0 {
		base.MaxFindings = local.MaxFindings
	}
	if local.MinAttackAngles > 0 {
		base.MinAttackAngles = local.MinAttackAngles
	}
	if local.ReviewDepth != "" {
		base.ReviewDepth = local.ReviewDepth
	}
	if local.RerunPolicy != "" {
		base.RerunPolicy = local.RerunPolicy
	}
	return base
}

func overlayPasses(base map[string]ReviewPassConfig, local map[string]ReviewPassConfig) map[string]ReviewPassConfig {
	if len(base) == 0 && len(local) == 0 {
		return nil
	}
	next := make(map[string]ReviewPassConfig, len(base)+len(local))
	for id, pass := range base {
		next[id] = pass
	}
	for id, pass := range local {
		current := next[id]
		if pass.Order != 0 {
			current.Order = pass.Order
		}
		if pass.Title != "" {
			current.Title = pass.Title
		}
		if pass.Description != "" {
			current.Description = pass.Description
		}
		next[id] = current
	}
	return next
}

func overlayStrings(base map[string]string, local map[string]string) map[string]string {
	if len(base) == 0 && len(local) == 0 {
		return nil
	}
	next := make(map[string]string, len(base)+len(local))
	for key, value := range base {
		next[key] = value
	}
	for key, value := range local {
		next[key] = value
	}
	return next
}

func dedupeList(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}

func withDefaults(cfg Config) Config {
	defaults := Default()
	if cfg.Version == "" {
		cfg.Version = defaults.Version
	}
	cfg.Invariants.Canonical = overlayStrings(defaults.Invariants.Canonical, cfg.Invariants.Canonical)
	if cfg.LLM.ModelProfile == "" {
		cfg.LLM.ModelProfile = defaults.LLM.ModelProfile
	}
	cfg.Execution = overlayExecution(defaults.Execution, cfg.Execution)
	cfg.Verify = overlayVerify(defaults.Verify, cfg.Verify)
	if cfg.Harden.MaxIssuesPerRound <= 0 {
		cfg.Harden.MaxIssuesPerRound = defaults.Harden.MaxIssuesPerRound
	}
	if cfg.Harden.ContextMaxBytes <= 0 {
		cfg.Harden.ContextMaxBytes = defaults.Harden.ContextMaxBytes
	}
	if cfg.Harden.RequiredContextMaxBytes <= 0 {
		cfg.Harden.RequiredContextMaxBytes = defaults.Harden.RequiredContextMaxBytes
	}
	cfg.Harden.External = overlayExternal(defaults.Harden.External, cfg.Harden.External)
	if cfg.Review.External.Provider == "" {
		cfg.Review.External.Provider = defaults.Review.External.Provider
	}
	if cfg.Review.External.IdleTimeoutSeconds <= 0 {
		cfg.Review.External.IdleTimeoutSeconds = defaults.Review.External.IdleTimeoutSeconds
	}
	if cfg.Review.External.AbsoluteMaxSeconds <= 0 {
		cfg.Review.External.AbsoluteMaxSeconds = defaults.Review.External.AbsoluteMaxSeconds
	}
	if cfg.Review.External.FallbackPolicy == "" {
		cfg.Review.External.FallbackPolicy = defaults.Review.External.FallbackPolicy
	}
	if cfg.Review.External.Codex.Model == "" {
		cfg.Review.External.Codex.Model = defaults.Review.External.Codex.Model
	}
	if cfg.Review.External.Claude.Model == "" {
		cfg.Review.External.Claude.Model = defaults.Review.External.Claude.Model
	}
	cfg.Review.AutomatedPasses = overlayPasses(defaults.Review.AutomatedPasses, cfg.Review.AutomatedPasses)
	cfg.Review.AdversarialPasses = overlayPasses(defaults.Review.AdversarialPasses, cfg.Review.AdversarialPasses)
	cfg.Review.Context = overlayReviewContext(defaults.Review.Context, cfg.Review.Context)
	cfg.Review.Dossier = overlayReviewDossier(defaults.Review.Dossier, cfg.Review.Dossier)
	cfg.Harden.External = normalizeProviderLatestModels(cfg.Harden.External)
	cfg.Review.External = normalizeProviderLatestModels(cfg.Review.External)
	return cfg
}

func normalizeProviderLatestModels(cfg ExternalReviewConfig) ExternalReviewConfig {
	cfg.Codex.Model = normalizeCodexLatestModel(cfg.Codex.Model)
	cfg.Codex.ModelReasoningEffort = normalizeCodexModelReasoningEffort(cfg.Codex.ModelReasoningEffort)
	cfg.Claude.Model = normalizeClaudeLatestModel(cfg.Claude.Model)
	cfg.Claude.Effort = normalizeClaudeEffort(cfg.Claude.Effort)
	return cfg
}

func normalizeCodexLatestModel(model string) string {
	return normalizeMappedModel(model, legacyCodexDefaultModels)
}

func normalizeCodexModelReasoningEffort(effort string) string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	switch normalized {
	case "", "latest", "current", "default":
		return DefaultCodexModelReasoningEffort
	default:
		return normalized
	}
}

func normalizeClaudeEffort(effort string) string {
	normalized := strings.ToLower(strings.TrimSpace(effort))
	switch normalized {
	case "", "latest", "current", "default":
		return DefaultClaudeEffort
	default:
		return normalized
	}
}

func normalizeMappedModel(model string, aliases map[string]string) string {
	if current, ok := aliases[strings.ToLower(strings.TrimSpace(model))]; ok {
		return current
	}
	return model
}

func normalizeClaudeLatestModel(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if normalized == "" {
		return model
	}
	if alias, ok := claudeLatestAliases[normalized]; ok {
		return alias
	}
	for _, family := range claudeLatestFamilies {
		if strings.HasPrefix(normalized, family+"-") || strings.HasPrefix(normalized, family+".") ||
			(strings.HasPrefix(normalized, "claude-") &&
				(strings.HasPrefix(normalized, "claude-"+family+"-") || strings.HasPrefix(normalized, "claude-"+family+".") ||
					strings.Contains(normalized, "-"+family+"-") || strings.Contains(normalized, "-"+family+"."))) {
			return claudeLatestAliases[family]
		}
	}
	return model
}

// ProcessEnv returns stable process environment overrides for acceptance commands.
func (cfg ExecutionConfig) ProcessEnv() []string {
	values := map[string]string{}
	for key, value := range cfg.Env {
		if strings.TrimSpace(key) != "" {
			values[key] = expandEnvValue(value)
		}
	}
	if len(cfg.PathPrepend) > 0 {
		current := os.Getenv("PATH")
		if values["PATH"] != "" {
			current = values["PATH"]
		}
		parts := make([]string, 0, len(cfg.PathPrepend)+1)
		for _, path := range cfg.PathPrepend {
			if expanded := expandPath(path); expanded != "" {
				parts = append(parts, expanded)
			}
		}
		if current != "" {
			parts = append(parts, current)
		}
		values["PATH"] = strings.Join(parts, string(os.PathListSeparator))
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

// AbsoluteTimeout returns the configured per-acceptance-command wall clock cap.
func (cfg ExecutionConfig) AbsoluteTimeout() time.Duration {
	if cfg.AbsoluteTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(cfg.AbsoluteTimeoutSeconds) * time.Second
}

// IdleTimeout returns the configured idle-output watchdog duration.
func (cfg ExecutionConfig) IdleTimeout() time.Duration {
	if cfg.IdleTimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(cfg.IdleTimeoutSeconds) * time.Second
}

func expandEnvValue(value string) string {
	return os.ExpandEnv(value)
}

func expandPath(value string) string {
	text := strings.TrimSpace(os.ExpandEnv(value))
	if text == "" {
		return ""
	}
	if text == "~" || strings.HasPrefix(text, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if text == "~" {
				return home
			}
			return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(text, "~/")))
		}
	}
	return text
}
