package corebundle

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

const gitignorePath = ".gitignore"

var scafldGitignoreBlock = []byte(`# scafld runtime state
# Keep project-owned specs/config/prompts/core in git; ignore local runtime state.
!.scafld/
!.scafld/config.yaml
!.scafld/core/
!.scafld/core/**
!.scafld/prompts/
!.scafld/prompts/**
!.scafld/specs/
!.scafld/specs/**
.scafld/config.local.yaml
.scafld/runs/
`)

// InitGitignore installs the scafld ignore contract into the workspace gitignore.
func InitGitignore(ctx context.Context, root string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	path := filepath.Join(root, gitignorePath)
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := atomicfile.Write(path, scafldGitignoreBlock, 0o644); err != nil {
			return Result{}, fmt.Errorf("write %s: %w", gitignorePath, err)
		}
		return Result{Created: []string{gitignorePath}}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("read %s: %w", gitignorePath, err)
	}
	next := installGitignoreBlock(current)
	if bytes.Equal(current, next) {
		return Result{Skipped: []string{gitignorePath}}, nil
	}
	if err := atomicfile.Write(path, next, 0o644); err != nil {
		return Result{}, fmt.Errorf("write %s: %w", gitignorePath, err)
	}
	return Result{Updated: []string{gitignorePath}}, nil
}

func installGitignoreBlock(current []byte) []byte {
	without := stripGitignoreBlock(current)
	trimmed := strings.TrimRight(string(without), "\n")
	if trimmed == "" {
		return scafldGitignoreBlock
	}
	return []byte(trimmed + "\n\n" + string(scafldGitignoreBlock))
}

func stripGitignoreBlock(current []byte) []byte {
	lines := strings.SplitAfter(string(current), "\n")
	var out strings.Builder
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "# scafld runtime state" {
			skipping = true
			continue
		}
		if skipping {
			if trimmed == "" {
				skipping = false
			}
			continue
		}
		out.WriteString(line)
	}
	return []byte(out.String())
}
