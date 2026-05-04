package bootstrap

import (
	"context"
	"errors"

	"github.com/nilstate/scafld/v2/internal/core/workspace"
)

var ErrMissingWorkspaceStore = errors.New("missing workspace store")

type WorkspaceStore interface {
	Init(ctx context.Context, root string) (workspace.InitResult, error)
}

type Input struct {
	Root string
}

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
