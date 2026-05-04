package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the merged runtime configuration for a scafld workspace.
type Config struct {
	Version string       `yaml:"version"`
	LLM     LLMConfig    `yaml:"llm"`
	Harden  HardenConfig `yaml:"harden"`
	Review  ReviewConfig `yaml:"review"`
}

// LLMConfig contains shared model-profile settings.
type LLMConfig struct {
	ModelProfile string `yaml:"model_profile"`
}

// HardenConfig controls hardening prompt behavior.
type HardenConfig struct {
	MaxQuestionsPerRound int `yaml:"max_questions_per_round"`
}

// ReviewConfig controls automated and adversarial review behavior.
type ReviewConfig struct {
	External          ExternalReviewConfig        `yaml:"external"`
	AutomatedPasses   map[string]ReviewPassConfig `yaml:"automated_passes"`
	AdversarialPasses map[string]ReviewPassConfig `yaml:"adversarial_passes"`
}

// ExternalReviewConfig configures external model-provider review execution.
type ExternalReviewConfig struct {
	Provider           string         `yaml:"provider"`
	Command            string         `yaml:"command"`
	ProviderBinary     string         `yaml:"provider_binary"`
	IdleTimeoutSeconds int            `yaml:"idle_timeout_seconds"`
	AbsoluteMaxSeconds int            `yaml:"absolute_max_seconds"`
	FallbackPolicy     string         `yaml:"fallback_policy"`
	Codex              ProviderConfig `yaml:"codex"`
	Claude             ProviderConfig `yaml:"claude"`
}

// ProviderConfig configures a named external provider implementation.
type ProviderConfig struct {
	Model  string `yaml:"model"`
	Binary string `yaml:"binary"`
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
	return overlay(cfg, local), nil
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
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Default returns the built-in workspace configuration.
func Default() Config {
	return Config{
		Version: "1.0",
		LLM:     LLMConfig{ModelProfile: "default"},
		Harden:  HardenConfig{MaxQuestionsPerRound: 8},
		Review: ReviewConfig{
			External: ExternalReviewConfig{
				Provider:           "auto",
				IdleTimeoutSeconds: 180,
				AbsoluteMaxSeconds: 1800,
				FallbackPolicy:     "warn",
				Codex:              ProviderConfig{Model: "gpt-5.5"},
				Claude:             ProviderConfig{Model: "claude-opus-4-7"},
			},
			AutomatedPasses: map[string]ReviewPassConfig{
				"spec_compliance": {Order: 10, Title: "Spec Compliance", Description: "Re-run acceptance criteria to verify code satisfies the spec"},
				"scope_drift":     {Order: 20, Title: "Scope Drift", Description: "Compare spec scope vs current workspace changes and flag undeclared changes"},
			},
			AdversarialPasses: map[string]ReviewPassConfig{
				"regression_hunt":  {Order: 30, Title: "Regression Hunt", Description: "Trace callers, importers, and downstream consumers for regressions"},
				"convention_check": {Order: 40, Title: "Convention Check", Description: "Check changed code against CONVENTIONS.md and AGENTS.md"},
				"dark_patterns":    {Order: 50, Title: "Dark Patterns", Description: "Hunt for subtle bugs, hardcodes, races, and safety gaps"},
			},
		},
	}
}

func overlay(base Config, local Config) Config {
	if local.Version != "" {
		base.Version = local.Version
	}
	if local.LLM.ModelProfile != "" {
		base.LLM.ModelProfile = local.LLM.ModelProfile
	}
	if local.Harden.MaxQuestionsPerRound > 0 {
		base.Harden.MaxQuestionsPerRound = local.Harden.MaxQuestionsPerRound
	}
	base.Review.External = overlayExternal(base.Review.External, local.Review.External)
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
	base.Codex = overlayProvider(base.Codex, local.Codex)
	base.Claude = overlayProvider(base.Claude, local.Claude)
	return base
}

func overlayProvider(base ProviderConfig, local ProviderConfig) ProviderConfig {
	if local.Model != "" {
		base.Model = local.Model
	}
	if local.Binary != "" {
		base.Binary = local.Binary
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

func withDefaults(cfg Config) Config {
	defaults := Default()
	if cfg.Version == "" {
		cfg.Version = defaults.Version
	}
	if cfg.LLM.ModelProfile == "" {
		cfg.LLM.ModelProfile = defaults.LLM.ModelProfile
	}
	if cfg.Harden.MaxQuestionsPerRound <= 0 {
		cfg.Harden.MaxQuestionsPerRound = defaults.Harden.MaxQuestionsPerRound
	}
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
	return cfg
}
