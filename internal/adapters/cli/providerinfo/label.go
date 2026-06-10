// Package providerinfo renders provider selection facts for CLI progress
// output shared by the review and harden subcommands.
package providerinfo

import (
	"os"
	"strings"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
)

// ProgressLabel renders "command[provider:model]" for live progress output,
// resolving the auto provider the same way the real selection will.
func ProgressLabel(command string, provider string, model string, binary string, external configadapter.ExternalReviewConfig) string {
	provider = First(provider, external.Provider, "auto")
	switch provider {
	case "command":
		return command + "[command]"
	case "local":
		return command + "[local]"
	case "claude":
		model = First(model, external.Claude.Model)
	case "codex":
		model = First(model, external.Codex.Model)
	case "gemini":
		model = First(model, external.Gemini.Model)
	default:
		selected, err := providers.AutoProviderInfo(providers.Selection{
			Binary:         binary,
			Model:          model,
			CodexModel:     external.Codex.Model,
			ClaudeModel:    external.Claude.Model,
			GeminiModel:    external.Gemini.Model,
			CodexBinary:    external.Codex.Binary,
			ClaudeBinary:   external.Claude.Binary,
			GeminiBinary:   external.Gemini.Binary,
			FallbackPolicy: external.FallbackPolicy,
			HostAgent:      providers.DetectHostAgent(os.Environ()),
		})
		if err != nil {
			provider = "auto"
		} else {
			provider = selected.Provider
			model = selected.Model
		}
	}
	if strings.TrimSpace(model) == "" {
		return command + "[" + provider + "]"
	}
	return command + "[" + provider + ":" + strings.TrimSpace(model) + "]"
}

// First returns the first value with non-blank content.
func First(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
