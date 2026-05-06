package corebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

//go:embed assets
var assets embed.FS

// Options controls how embedded core assets are installed.
type Options struct {
	OverwriteCore         bool
	CreateProjectPrompts  bool
	RefreshProjectPrompts bool
	CreateProjectConfig   bool
}

// Result summarizes files created, updated, or skipped during installation.
type Result struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
}

// Init installs managed core assets for a newly bootstrapped workspace.
func Init(ctx context.Context, root string) (Result, error) {
	return Install(ctx, root, Options{
		OverwriteCore:        false,
		CreateProjectPrompts: true,
		CreateProjectConfig:  true,
	})
}

// Update refreshes managed assets and default project prompt copies.
func Update(ctx context.Context, root string) (Result, error) {
	return Install(ctx, root, Options{OverwriteCore: true, RefreshProjectPrompts: true})
}

// Install copies embedded assets into root according to opts.
func Install(ctx context.Context, root string, opts Options) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	var result Result
	if err := installTree(ctx, root, "assets/core", ".scafld/core", opts.OverwriteCore, &result); err != nil {
		return Result{}, err
	}
	if opts.CreateProjectPrompts {
		if err := installProjectPrompts(ctx, root, false, &result); err != nil {
			return Result{}, err
		}
	}
	if opts.RefreshProjectPrompts {
		if err := installProjectPrompts(ctx, root, true, &result); err != nil {
			return Result{}, err
		}
	}
	if opts.CreateProjectConfig {
		if err := installProjectConfig(ctx, root, &result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
}

type promptManifest struct {
	Version int               `json:"version"`
	Prompts map[string]string `json:"prompts"`
}

func installProjectPrompts(ctx context.Context, root string, refresh bool, result *Result) error {
	manifest := loadPromptManifest(root)
	changedManifest := false
	err := fs.WalkDir(assets, "assets/prompts", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel("assets/prompts", path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		targetRel := filepath.ToSlash(filepath.Join(".scafld/prompts", rel))
		target := filepath.Join(root, filepath.FromSlash(targetRel))
		data, err := assets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		changed, err := writeProjectPrompt(target, targetRel, rel, data, refresh, manifest, result)
		if err != nil {
			return err
		}
		changedManifest = changedManifest || changed
		return nil
	})
	if err != nil {
		return err
	}
	if changedManifest || len(manifest.Prompts) > 0 {
		return writePromptManifest(root, manifest)
	}
	return nil
}

func writeProjectPrompt(path string, targetRel string, rel string, data []byte, refresh bool, manifest promptManifest, result *Result) (bool, error) {
	if manifest.Prompts == nil {
		manifest.Prompts = map[string]string{}
	}
	targetHash := sha256Hex(data)
	existing, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if !os.IsNotExist(err) {
			return false, fmt.Errorf("read %s: %w", targetRel, err)
		}
		if err := writeManagedFile(path, targetRel, data, false, result); err != nil {
			return false, err
		}
		manifest.Prompts[rel] = targetHash
		return true, nil
	}
	existingHash := sha256Hex(existing)
	if bytes.Equal(existing, data) {
		result.Skipped = append(result.Skipped, targetRel)
		if manifest.Prompts[rel] != targetHash {
			manifest.Prompts[rel] = targetHash
			return true, nil
		}
		return false, nil
	}
	if refresh {
		if manifest.Prompts[rel] != "" && manifest.Prompts[rel] == existingHash {
			if err := atomicfile.Write(path, data, fileMode(targetRel)); err != nil {
				return false, fmt.Errorf("write %s: %w", targetRel, err)
			}
			result.Updated = append(result.Updated, targetRel)
			manifest.Prompts[rel] = targetHash
			return true, nil
		}
		if migrated, ok := migrateLegacyProjectPrompt(rel, existing, data); ok && !bytes.Equal(migrated, existing) {
			if err := atomicfile.Write(path, migrated, fileMode(targetRel)); err != nil {
				return false, fmt.Errorf("write %s: %w", targetRel, err)
			}
			result.Updated = append(result.Updated, targetRel)
			if bytes.Equal(migrated, data) {
				manifest.Prompts[rel] = targetHash
			} else {
				delete(manifest.Prompts, rel)
			}
			return true, nil
		}
	}
	result.Skipped = append(result.Skipped, targetRel)
	return false, nil
}

func loadPromptManifest(root string) promptManifest {
	path := filepath.Join(root, ".scafld", "prompts", ".manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return promptManifest{Version: 1, Prompts: map[string]string{}}
	}
	var manifest promptManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return promptManifest{Version: 1, Prompts: map[string]string{}}
	}
	if manifest.Version == 0 {
		manifest.Version = 1
	}
	if manifest.Prompts == nil {
		manifest.Prompts = map[string]string{}
	}
	return manifest
}

func writePromptManifest(root string, manifest promptManifest) error {
	if manifest.Version == 0 {
		manifest.Version = 1
	}
	if manifest.Prompts == nil {
		manifest.Prompts = map[string]string{}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prompt manifest: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(root, ".scafld", "prompts", ".manifest.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for .scafld/prompts/.manifest.json: %w", err)
	}
	return atomicfile.Write(path, data, 0o644)
}

func migrateLegacyProjectPrompt(rel string, existing []byte, current []byte) ([]byte, bool) {
	switch rel {
	case "review.md":
		return migrateLegacyReviewPrompt(string(existing), string(current))
	case "harden.md":
		return migrateLegacyHardenPrompt(string(existing))
	default:
		return existing, false
	}
}

func migrateLegacyReviewPrompt(existing string, current string) ([]byte, bool) {
	if !strings.Contains(existing, ".scafld/reviews/") &&
		!strings.Contains(existing, ".ai/reviews/") &&
		!strings.Contains(existing, "fill only the latest review round") &&
		!strings.Contains(existing, "pass_with_issues") {
		return []byte(existing), false
	}
	updated := existing
	for _, heading := range []string{"## Attack Plan", "## Output Contract", "## Verdict Rules"} {
		replacement, ok := markdownSection(current, heading)
		if !ok {
			continue
		}
		var changed bool
		updated, changed = replaceMarkdownSection(updated, heading, replacement)
		if !changed {
			return []byte(existing), false
		}
	}
	return []byte(updated), updated != existing
}

func migrateLegacyHardenPrompt(existing string) ([]byte, bool) {
	updated := existing
	updated = strings.ReplaceAll(updated, "single `grounded_in` value", "single `Grounded in:` value")
	updated = strings.ReplaceAll(updated, "Use `grounded_in` as", "Use `Grounded in:` as")
	updated = strings.ReplaceAll(updated, "include `if_unanswered` with", "include `If unanswered:` with")
	updated = strings.Replace(
		updated,
		"Record each question in this Markdown shape under the latest harden round:",
		"Record each question in this exact Markdown shape under the latest harden round.\nDo not use YAML object keys such as `question:`, `grounded_in:`, `recommended_answer:`, or `resolution:`.",
		1,
	)
	return []byte(updated), updated != existing
}

func markdownSection(text string, heading string) (string, bool) {
	start, end, ok := markdownSectionBounds(text, heading)
	if !ok {
		return "", false
	}
	return strings.TrimRight(text[start:end], "\n") + "\n\n", true
}

func markdownSectionBounds(text string, heading string) (int, int, bool) {
	start := strings.Index(text, heading)
	if start < 0 {
		return 0, 0, false
	}
	after := start + len(heading)
	end := len(text)
	if next := strings.Index(text[after:], "\n## "); next >= 0 {
		end = after + next + 1
	}
	return start, end, true
}

func replaceMarkdownSection(text string, heading string, replacement string) (string, bool) {
	start, end, ok := markdownSectionBounds(text, heading)
	if !ok {
		return text, false
	}
	replacement = strings.TrimRight(replacement, "\n") + "\n"
	if end < len(text) {
		replacement += "\n"
	}
	return text[:start] + replacement + text[end:], true
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func installTree(ctx context.Context, root string, source string, dest string, overwrite bool, result *Result) error {
	return fs.WalkDir(assets, source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		targetRel := filepath.ToSlash(filepath.Join(dest, rel))
		target := filepath.Join(root, filepath.FromSlash(targetRel))
		data, err := assets.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		return writeManagedFile(target, targetRel, data, overwrite, result)
	})
}

func installProjectConfig(ctx context.Context, root string, result *Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := assets.ReadFile("assets/core/config.yaml")
	if err != nil {
		return fmt.Errorf("read embedded config: %w", err)
	}
	if err := writeManagedFile(filepath.Join(root, ".scafld", "config.yaml"), ".scafld/config.yaml", data, false, result); err != nil {
		return err
	}
	local := []byte("# Local scafld overrides. This file should stay uncommitted.\n# review:\n#   external:\n#     provider: \"codex\"\n#     codex:\n#       model: \"gpt-5.5\"\n")
	return writeManagedFile(filepath.Join(root, ".scafld", "config.local.yaml"), ".scafld/config.local.yaml", local, false, result)
}

func writeManagedFile(path string, rel string, data []byte, overwrite bool, result *Result) error {
	exists := false
	if _, err := os.Stat(path); err == nil {
		exists = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", rel, err)
	}
	if exists && !overwrite {
		result.Skipped = append(result.Skipped, rel)
		return nil
	}
	if exists {
		current, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		if bytes.Equal(current, data) {
			result.Skipped = append(result.Skipped, rel)
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir for %s: %w", rel, err)
	}
	if err := atomicfile.Write(path, data, fileMode(rel)); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	if exists {
		result.Updated = append(result.Updated, rel)
	} else {
		result.Created = append(result.Created, rel)
	}
	return nil
}

func fileMode(rel string) os.FileMode {
	if strings.Contains(rel, "/scripts/") {
		return 0o755
	}
	return 0o644
}
