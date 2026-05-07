package review

import (
	"context"
	"fmt"
	"io"
	osexec "os/exec"
	"strings"
	"time"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
)

// Options configures review-provider selection for the CLI.
type Options struct {
	Root            string
	TaskID          string
	Provider        string
	Command         string
	ProviderBinary  string
	Model           string
	DiagnosticsPath string
	Progress        io.Writer
}

// Selection is the provider and review agenda chosen for a review run.
type Selection struct {
	Provider appreview.Provider
	Passes   []appreview.Pass
}

// Select loads config, applies CLI overrides, and returns a review provider.
func Select(ctx context.Context, opts Options) (Selection, error) {
	cfg, err := configadapter.Load(ctx, opts.Root)
	if err != nil {
		return Selection{}, fmt.Errorf("load config: %w", err)
	}
	external := cfg.Review.External
	diagnosticsPath := opts.DiagnosticsPath
	if diagnosticsPath == "" {
		diagnosticsPath = opts.Root + "/.scafld/runs/" + opts.TaskID + "/diagnostics"
	}
	provider, err := providers.Select(providers.Selection{
		Provider:       first(opts.Provider, external.Provider),
		Command:        first(opts.Command, external.Command),
		Binary:         first(opts.ProviderBinary, external.ProviderBinary),
		Model:          opts.Model,
		CodexModel:     external.Codex.Model,
		ClaudeModel:    external.Claude.Model,
		CodexBinary:    external.Codex.Binary,
		ClaudeBinary:   external.Claude.Binary,
		CWD:            opts.Root,
		Runner:         process.Runner{DiagnosticsDir: diagnosticsPath, Progress: opts.Progress, ProgressLabel: progressLabel(opts, external)},
		Timeout:        time.Duration(external.AbsoluteMaxSeconds) * time.Second,
		Idle:           time.Duration(external.IdleTimeoutSeconds) * time.Second,
		FallbackPolicy: external.FallbackPolicy,
	})
	if err != nil {
		return Selection{}, err
	}
	return Selection{Provider: provider, Passes: reviewPassesFromConfig(cfg.Review)}, nil
}

func progressLabel(opts Options, external configadapter.ExternalReviewConfig) string {
	provider := first(opts.Provider, external.Provider, "auto")
	model := opts.Model
	switch provider {
	case "command":
		return "review[command]"
	case "local":
		return "review[local]"
	case "claude":
		model = first(model, external.Claude.Model)
	case "codex":
		model = first(model, external.Codex.Model)
	default:
		switch {
		case opts.ProviderBinary != "":
			provider = "codex"
			model = first(model, external.Codex.Model)
		case commandExists("codex"):
			provider = "codex"
			model = first(model, external.Codex.Model)
		case commandExists("claude"):
			provider = "claude"
			model = first(model, external.Claude.Model)
		default:
			provider = "auto"
		}
	}
	if strings.TrimSpace(model) == "" {
		return "review[" + provider + "]"
	}
	return "review[" + provider + ":" + strings.TrimSpace(model) + "]"
}

func commandExists(name string) bool {
	_, err := osexec.LookPath(name)
	return err == nil
}

func reviewPassesFromConfig(cfg configadapter.ReviewConfig) []appreview.Pass {
	passes := make([]appreview.Pass, 0, len(cfg.AutomatedPasses)+len(cfg.AdversarialPasses))
	for id, pass := range cfg.AutomatedPasses {
		passes = append(passes, appreview.Pass{ID: id, Category: "automated", Order: pass.Order, Title: pass.Title, Description: pass.Description})
	}
	for id, pass := range cfg.AdversarialPasses {
		passes = append(passes, appreview.Pass{ID: id, Category: "adversarial", Order: pass.Order, Title: pass.Title, Description: pass.Description})
	}
	return passes
}

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
