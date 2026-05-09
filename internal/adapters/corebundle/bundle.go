package corebundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
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
	if opts.OverwriteCore {
		if err := normalizeProjectConfig(ctx, root, &result); err != nil {
			return Result{}, err
		}
	}
	return result, nil
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

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
