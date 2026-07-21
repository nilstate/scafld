package review

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	contractloader "github.com/nilstate/scafld/v2/internal/adapters/cli/agentcontract"
	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	"github.com/nilstate/scafld/v2/internal/adapters/cli/providerinfo"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	corecontract "github.com/nilstate/scafld/v2/internal/core/agentcontract"
	"github.com/nilstate/scafld/v2/internal/core/reviewcontext"
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
	PrintContext    bool
}

// Selection is the provider and review agenda chosen for a review run.
type Selection struct {
	Provider                appreview.Provider
	ProviderName            string
	ProviderModel           string
	Passes                  []appreview.Pass
	Invariants              map[string]string
	ContextSections         []reviewcontext.Section
	ContextMaxBytes         int
	RequiredContextMaxBytes int
	Contract                corecontract.Contract
	Dossier                 configadapter.ReviewDossierConfig
}

// Select loads config, applies CLI overrides, and returns a review provider.
func Select(ctx context.Context, opts Options) (Selection, error) {
	cfg, err := configadapter.Load(ctx, opts.Root)
	if err != nil {
		return Selection{}, output.ConfigGateError(fmt.Errorf("load config: %w", err))
	}
	contextSections := reviewContextSections(opts.Root, cfg.Review.Context)
	contract, err := contractloader.Load(ctx, opts.Root, corecontract.RoleReview)
	if err != nil {
		return Selection{}, output.ConfigGateError(fmt.Errorf("load review contract: %w", err))
	}
	selection := Selection{
		Passes:                  reviewPassesFromConfig(cfg.Review),
		Invariants:              cloneStrings(cfg.Invariants.Canonical),
		ContextSections:         contextSections,
		ContextMaxBytes:         cfg.Review.Context.MaxBytes,
		RequiredContextMaxBytes: cfg.Review.Context.RequiredMaxBytes,
		Contract:                contract,
		Dossier:                 cfg.Review.Dossier,
	}
	if opts.PrintContext {
		return selection, nil
	}
	external := cfg.Review.External
	diagnosticsPath := opts.DiagnosticsPath
	if diagnosticsPath == "" {
		diagnosticsPath = opts.Root + "/.scafld/runs/" + opts.TaskID + "/diagnostics"
	}
	provider, err := providers.Select(providers.Selection{
		Provider:                  providerinfo.First(opts.Provider, external.Provider),
		Command:                   providerinfo.First(opts.Command, external.Command),
		Binary:                    providerinfo.First(opts.ProviderBinary, external.ProviderBinary),
		Model:                     opts.Model,
		CodexModel:                external.Codex.Model,
		CodexModelReasoningEffort: external.Codex.ModelReasoningEffort,
		ClaudeModel:               external.Claude.Model,
		ClaudeEffort:              external.Claude.Effort,
		GeminiModel:               external.Gemini.Model,
		CodexBinary:               external.Codex.Binary,
		ClaudeBinary:              external.Claude.Binary,
		GeminiBinary:              external.Gemini.Binary,
		CWD:                       opts.Root,
		Runner:                    process.Runner{DiagnosticsDir: diagnosticsPath, Progress: opts.Progress, ProgressLabel: progressLabel(opts, external)},
		Timeout:                   time.Duration(external.AbsoluteMaxSeconds) * time.Second,
		Idle:                      time.Duration(external.IdleTimeoutSeconds) * time.Second,
		FallbackPolicy:            external.FallbackPolicy,
		HostAgent:                 providers.DetectHostAgent(os.Environ()),
	})
	if err != nil {
		return Selection{}, output.ReviewProviderGateError(err)
	}
	selection.Provider = provider
	selection.ProviderName, selection.ProviderModel = providerFacts(provider)
	return selection, nil
}

func providerFacts(provider appreview.Provider) (string, string) {
	switch p := provider.(type) {
	case providers.CommandProvider:
		return "command", ""
	case providers.LocalProvider:
		return "local", ""
	case providers.CodexProvider:
		return "codex", p.Model
	case providers.ClaudeProvider:
		return "claude", p.Model
	case providers.GeminiProvider:
		return "gemini", p.Model
	default:
		return "", ""
	}
}

func progressLabel(opts Options, external configadapter.ExternalReviewConfig) string {
	return providerinfo.ProgressLabel("review", opts.Provider, opts.Model, opts.ProviderBinary, external)
}

func cloneStrings(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
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

func reviewContextSections(root string, cfg configadapter.ReviewContextConfig) []reviewcontext.Section {
	sections := make([]reviewcontext.Section, 0, len(cfg.Files))
	for index, rel := range cfg.Files {
		clean, ok := safeContextPath(rel)
		if !ok {
			continue
		}
		sections = append(sections, reviewContextSectionsForPath(root, clean, 1000+index*100)...)
	}
	return sections
}

func reviewContextSectionsForPath(root string, clean string, order int) []reviewcontext.Section {
	info, ok := safeContextInfo(root, clean)
	if !ok {
		return nil
	}
	if !info.IsDir() {
		data, ok := readSafeContextFile(root, clean)
		if !ok {
			return nil
		}
		return []reviewcontext.Section{contextFileSection(clean, order, data)}
	}
	files := contextDirectoryFiles(root, clean)
	sections := make([]reviewcontext.Section, 0, len(files))
	for index, file := range files {
		data, ok := readSafeContextFile(root, file)
		if !ok {
			continue
		}
		sections = append(sections, contextFileSection(file, order+index, data))
	}
	return sections
}

func contextFileSection(clean string, order int, data []byte) reviewcontext.Section {
	return reviewcontext.Section{
		Key:     "file:" + clean,
		Title:   "Project Context: " + clean,
		Order:   order,
		Body:    string(data),
		Sources: []reviewcontext.Source{reviewcontext.SourceForContent("file", clean, data)},
	}
}

func contextDirectoryFiles(root string, clean string) []string {
	base := filepath.Join(root, filepath.FromSlash(clean))
	var files []string
	_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel, ok := safeContextPath(rel); ok && contextRuleFile(rel) {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func contextRuleFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".mdc", ".txt":
		return true
	default:
		return false
	}
}

func safeContextInfo(root string, clean string) (os.FileInfo, bool) {
	path := filepath.Join(root, filepath.FromSlash(clean))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, false
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, false
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return nil, false
	}
	rel = filepath.ToSlash(rel)
	if _, ok := safeContextPath(rel); !ok || rel != clean {
		return nil, false
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, false
	}
	return info, true
}

func readSafeContextFile(root string, clean string) ([]byte, bool) {
	path := filepath.Join(root, filepath.FromSlash(clean))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, false
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, false
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil {
		return nil, false
	}
	rel = filepath.ToSlash(rel)
	if _, ok := safeContextPath(rel); !ok || rel != clean {
		return nil, false
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, false
	}
	return data, true
}

func safeContextPath(path string) (string, bool) {
	raw := strings.ReplaceAll(strings.TrimSpace(path), "\\", "/")
	clean := strings.Trim(filepath.ToSlash(filepath.Clean(raw)), "/")
	root := strings.Split(clean, "/")[0]
	if clean == "." || clean == "" || strings.HasPrefix(raw, "/") || strings.Contains(root, ":") || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", false
	}
	switch {
	case clean == ".scafld/config.local.yaml":
		return "", false
	case hasPrivateEnvSegment(clean):
		return "", false
	case strings.HasPrefix(clean+"/", ".git/"):
		return "", false
	case strings.HasPrefix(clean+"/", ".priv/"):
		return "", false
	default:
		return clean, true
	}
}

func hasPrivateEnvSegment(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if strings.HasPrefix(segment, ".env") {
			return true
		}
	}
	return false
}
