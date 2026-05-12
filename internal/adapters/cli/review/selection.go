package review

import (
	"context"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nilstate/scafld/v2/internal/adapters/cli/output"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	"github.com/nilstate/scafld/v2/internal/adapters/process"
	"github.com/nilstate/scafld/v2/internal/adapters/providers"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
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
	Provider        appreview.Provider
	Passes          []appreview.Pass
	Invariants      map[string]string
	ContextSections []reviewcontext.Section
	ContextMaxBytes int
	Dossier         configadapter.ReviewDossierConfig
}

// Select loads config, applies CLI overrides, and returns a review provider.
func Select(ctx context.Context, opts Options) (Selection, error) {
	cfg, err := configadapter.Load(ctx, opts.Root)
	if err != nil {
		return Selection{}, output.ConfigGateError(fmt.Errorf("load config: %w", err))
	}
	contextSections := reviewContextSections(opts.Root, cfg.Review.Context)
	selection := Selection{
		Passes:          reviewPassesFromConfig(cfg.Review),
		Invariants:      cloneStrings(cfg.Invariants.Canonical),
		ContextSections: contextSections,
		ContextMaxBytes: cfg.Review.Context.MaxBytes,
		Dossier:         cfg.Review.Dossier,
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
		return Selection{}, output.ReviewProviderGateError(err)
	}
	selection.Provider = provider
	return selection, nil
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

func first(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
