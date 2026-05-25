package corebundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

type promptManifest struct {
	Version int               `json:"version"`
	Prompts map[string]string `json:"prompts"`
}

func installProjectPrompts(ctx context.Context, root string, refresh bool, result *Result) error {
	manifest := loadPromptManifest(root)
	changedManifest := false
	err := fs.WalkDir(assets, "assets/core/prompts", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel("assets/core/prompts", path)
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
		if refresh {
			return false, nil
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
