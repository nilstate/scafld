package harden

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	"github.com/nilstate/scafld/v2/internal/adapters/cli/providerinfo"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	appharden "github.com/nilstate/scafld/v2/internal/app/harden"
)

// Options configures harden-provider selection for the CLI.
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

// Selection is the provider and context budget chosen for a harden run.
type Selection struct {
	Provider        appharden.Provider
	ContextMaxBytes int
}

// RunOptions configures the app harden input assembled by the CLI adapter.
type RunOptions struct {
	Root           string
	TaskID         string
	MarkPassed     bool
	Provider       string
	Command        string
	ProviderBinary string
	Model          string
	Progress       io.Writer
}

// BuildInput returns the app-layer harden input for CLI execution.
func BuildInput(ctx context.Context, opts RunOptions) (appharden.Input, error) {
	selected, err := Select(ctx, Options{
		Root:           opts.Root,
		TaskID:         opts.TaskID,
		Provider:       opts.Provider,
		Command:        opts.Command,
		ProviderBinary: opts.ProviderBinary,
		Model:          opts.Model,
		Progress:       opts.Progress,
	})
	if err != nil {
		return appharden.Input{}, err
	}
	return appharden.Input{
		TaskID:          opts.TaskID,
		MarkPassed:      opts.MarkPassed,
		Root:            opts.Root,
		Prompt:          Prompt(ctx, opts.Root),
		Provider:        selected.Provider,
		ContextMaxBytes: selected.ContextMaxBytes,
	}, nil
}

// ResultText formats non-JSON harden output. The boolean reports whether the
// text should be wrapped in the standard success envelope.
func ResultText(stderr io.Writer, out appharden.Output) (string, bool) {
	if out.MarkedPassed {
		for _, warning := range out.Warnings {
			fmt.Fprintf(stderr, "warn: %s\n", warning)
		}
		return fmt.Sprintf("harden passed: %s\nnext: %s\n", out.TaskID, out.NextCommand), true
	}
	if out.Verdict != "" {
		if out.Summary != "" {
			return fmt.Sprintf("harden %s: %s\nsummary: %s\nnext: %s\n", out.Verdict, out.TaskID, out.Summary, out.NextCommand), true
		}
		return fmt.Sprintf("harden %s: %s\nnext: %s\n", out.Verdict, out.TaskID, out.NextCommand), true
	}
	return out.Prompt + fmt.Sprintf("\n---\nspec: %s\nround: %s\nwhen done, mark the round passed: %s\n", out.Path, out.RoundID, out.NextCommand), false
}

// Select loads config, applies CLI overrides, and returns a harden provider.
func Select(ctx context.Context, opts Options) (Selection, error) {
	cfg, err := configadapter.Load(ctx, opts.Root)
	if err != nil {
		return Selection{}, output.ConfigGateError(fmt.Errorf("load config: %w", err))
	}
	selection := Selection{ContextMaxBytes: cfg.Harden.ContextMaxBytes}
	external := cfg.Harden.External
	if !hardenProviderRequested(opts, external) {
		return selection, nil
	}
	diagnosticsPath := opts.DiagnosticsPath
	if diagnosticsPath == "" {
		diagnosticsPath = opts.Root + "/.scafld/runs/" + opts.TaskID + "/diagnostics"
	}
	provider, err := providers.SelectHarden(providers.Selection{
		Provider:       providerinfo.First(opts.Provider, external.Provider),
		Command:        providerinfo.First(opts.Command, external.Command),
		Binary:         providerinfo.First(opts.ProviderBinary, external.ProviderBinary),
		Model:          opts.Model,
		CodexModel:     external.Codex.Model,
		ClaudeModel:    external.Claude.Model,
		GeminiModel:    external.Gemini.Model,
		CodexBinary:    external.Codex.Binary,
		ClaudeBinary:   external.Claude.Binary,
		GeminiBinary:   external.Gemini.Binary,
		CWD:            opts.Root,
		Runner:         process.Runner{DiagnosticsDir: diagnosticsPath, Progress: opts.Progress, ProgressLabel: progressLabel(opts, external)},
		Timeout:        time.Duration(external.AbsoluteMaxSeconds) * time.Second,
		Idle:           time.Duration(external.IdleTimeoutSeconds) * time.Second,
		FallbackPolicy: external.FallbackPolicy,
		HostAgent:      providers.DetectHostAgent(os.Environ()),
	})
	if err != nil {
		return Selection{}, output.ReviewProviderGateError(err)
	}
	selection.Provider = provider
	return selection, nil
}

func hardenProviderRequested(opts Options, external configadapter.ExternalReviewConfig) bool {
	return strings.TrimSpace(opts.Provider) != "" ||
		strings.TrimSpace(opts.Command) != "" ||
		strings.TrimSpace(opts.ProviderBinary) != "" ||
		strings.TrimSpace(external.Provider) != ""
}

func progressLabel(opts Options, external configadapter.ExternalReviewConfig) string {
	return providerinfo.ProgressLabel("harden", opts.Provider, opts.Model, opts.ProviderBinary, external)
}
