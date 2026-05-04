package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/nilstate/scafld/v2/internal/core/workspace"
)

type fakeWorkspaceStore struct {
	root string
}

func (f *fakeWorkspaceStore) Init(ctx context.Context, root string) (workspace.InitResult, error) {
	if ctx == nil {
		return workspace.InitResult{}, errors.New("nil context")
	}
	f.root = root
	return workspace.InitResult{Root: root, Created: []string{".scafld"}}, nil
}

func TestRunUsesCurrentDirectoryByDefault(t *testing.T) {
	t.Parallel()

	store := &fakeWorkspaceStore{}
	result, err := Run(context.Background(), store, Input{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Root != "." {
		t.Fatalf("root = %q, want .", result.Root)
	}
	if store.root != "." {
		t.Fatalf("store root = %q, want .", store.root)
	}
}

func TestRunRequiresWorkspaceStore(t *testing.T) {
	t.Parallel()

	_, err := Run(context.Background(), nil, Input{})
	if !errors.Is(err, ErrMissingWorkspaceStore) {
		t.Fatalf("error = %v, want %v", err, ErrMissingWorkspaceStore)
	}
}
