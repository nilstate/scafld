package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceDiscoveryHonorsRootEnvAndWalkUp(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	child := filepath.Join(parent, "a", "b")
	if err := os.MkdirAll(filepath.Join(parent, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	explicit := t.TempDir()
	got, err := ResolveRoot(context.Background(), explicit, Discovery{})
	if err != nil || got != explicit {
		t.Fatalf("explicit root = %s, %v", got, err)
	}
	got, err = ResolveRoot(context.Background(), "", Discovery{EnvRoot: parent})
	if err != nil || got != parent {
		t.Fatalf("env root = %s, %v", got, err)
	}
	got, err = ResolveRoot(context.Background(), "", Discovery{CWD: child})
	if err != nil || got != parent {
		t.Fatalf("walk root = %s, %v", got, err)
	}
}
