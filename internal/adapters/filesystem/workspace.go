package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nilstate/scafld/v2/internal/core/workspace"
)

// WorkspaceStore creates the project-owned workspace directory layout.
type WorkspaceStore struct{}

// Discovery controls workspace root discovery.
type Discovery struct {
	EnvRoot string
	CWD     string
}

// Init creates the .scafld workspace directories under root.
func (WorkspaceStore) Init(ctx context.Context, root string) (workspace.InitResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return workspace.InitResult{}, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return workspace.InitResult{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	created := make([]string, 0, 8)
	for _, rel := range []string{
		".scafld",
		".scafld/core",
		".scafld/prompts",
		".scafld/specs",
		".scafld/specs/drafts",
		".scafld/specs/approved",
		".scafld/specs/active",
		".scafld/runs",
	} {
		if err := ctx.Err(); err != nil {
			return workspace.InitResult{}, err
		}
		path := filepath.Join(abs, filepath.FromSlash(rel))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			created = append(created, rel)
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return workspace.InitResult{}, fmt.Errorf("create %s: %w", rel, err)
		}
	}
	return workspace.InitResult{Root: abs, Created: created}, nil
}

// ResolveRoot resolves an explicit, environment, or walk-up workspace root.
func ResolveRoot(ctx context.Context, explicit string, opts Discovery) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	envRoot := opts.EnvRoot
	if envRoot == "" {
		envRoot = os.Getenv("SCAFLD_ROOT")
	}
	if envRoot != "" {
		return filepath.Abs(envRoot)
	}
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get cwd: %w", err)
		}
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", fmt.Errorf("resolve cwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(abs, ".scafld")); err == nil {
			return abs, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", fmt.Errorf("workspace not found from %s", cwd)
		}
		abs = parent
	}
}
