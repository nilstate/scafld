package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/nilstate/scafld/v2/internal/core/workspace"
)

// ErrMissingWorkspaceStore is returned when bootstrap has no workspace port.
var ErrMissingWorkspaceStore = errors.New("missing workspace store")

// WorkspaceStore creates workspace directories.
type WorkspaceStore interface {
	Init(ctx context.Context, root string) (workspace.InitResult, error)
}

// Input describes the workspace root to bootstrap.
type Input struct {
	Root string
}

// Run creates the scafld workspace layout.
func Run(ctx context.Context, store WorkspaceStore, input Input) (workspace.InitResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if store == nil {
		return workspace.InitResult{}, ErrMissingWorkspaceStore
	}
	root := input.Root
	if root == "" {
		root = "."
	}
	return store.Init(ctx, root)
}

// Message renders the human-facing summary for an init result.
func Message(result workspace.InitResult) string {
	switch {
	case !result.Changed():
		return fmt.Sprintf("scafld workspace already initialized: %s\n", result.Root)
	case len(result.Created) == 0:
		return fmt.Sprintf("updated scafld workspace: %s\n", result.Root)
	default:
		return fmt.Sprintf("initialized scafld workspace: %s\n", result.Root)
	}
}
