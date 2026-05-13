package corebundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

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
		return migrateLegacyHardenPrompt(string(existing), string(current))
	default:
		return existing, false
	}
}

func migrateLegacyReviewPrompt(existing string, current string) ([]byte, bool) {
	if !strings.Contains(existing, "fill only the latest review round") &&
		!strings.Contains(existing, "pass_with_issues") &&
		!strings.Contains(existing, "."+"scafld/reviews") {
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

func migrateLegacyHardenPrompt(existing string, current string) ([]byte, bool) {
	if !isLegacyHardenPrompt(existing) {
		return []byte(existing), false
	}
	updated := existing
	if !strings.Contains(updated, "Path audit:") {
		block := hardenChecksBlock(current)
		if block != "" {
			next := strings.Replace(
				updated,
				"Work these harden questions before polishing wording:",
				block+"\nWork these harden questions after the checks expose the real uncertainty:",
				1,
			)
			if next == updated {
				next = block + "\n\n" + updated
			}
			updated = next
		}
	}
	updated = strings.ReplaceAll(updated, "single `grounded_in` value", "single `Grounded in:` value")
	updated = strings.ReplaceAll(updated, "Use `grounded_in` as", "Use `Grounded in:` as")
	updated = strings.ReplaceAll(updated, "include `if_unanswered` with", "include `If unanswered:` with")
	updated = strings.Replace(
		updated,
		"If you cannot form a genuine grounded question, stop. Do not pad the round.",
		"If the checks pass and you cannot form a genuine grounded question, record:\n\n```markdown\nQuestions:\n- none\n```\n\nDo not pad the round.",
		1,
	)
	if !strings.Contains(updated, "Questions:\n- none") {
		updated = strings.Replace(
			updated,
			"Record each question in this Markdown shape under the latest harden round:",
			"If the checks pass and you cannot form a genuine grounded question, record:\n\n```markdown\nQuestions:\n- none\n```\n\nDo not pad the round.\n\nRecord each question in this Markdown shape under the latest harden round:",
			1,
		)
	}
	updated = strings.Replace(
		updated,
		"Record each question in this Markdown shape under the latest harden round:",
		"Record each question in this exact Markdown shape under the latest harden round.\nDo not use YAML object keys such as `question:`, `grounded_in:`, `recommended_answer:`, or `resolution:`.",
		1,
	)
	return []byte(updated), updated != existing
}

func isLegacyHardenPrompt(existing string) bool {
	return strings.Contains(existing, "grounded_in") ||
		strings.Contains(existing, "Record each question in this Markdown shape under the latest harden round:") ||
		strings.Contains(existing, "If you cannot form a genuine grounded question, stop.")
}

func hardenChecksBlock(current string) string {
	start := strings.Index(current, "Run these checks before polishing wording:")
	end := strings.Index(current, "Work these harden questions after the checks expose the real uncertainty:")
	if start < 0 || end <= start {
		return ""
	}
	return strings.TrimSpace(current[start:end])
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
