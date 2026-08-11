package runartifact

import (
	"path/filepath"
	"testing"
)

func TestRuntimeArtifactPathsAreSeparatedByRole(t *testing.T) {
	t.Parallel()

	root := "/repo"
	if got, want := SessionPath(root, "task"), filepath.Join(root, ".scafld", "runs", "task", "session.json"); got != want {
		t.Fatalf("SessionPath = %q, want %q", got, want)
	}
	if got, want := CommandDiagnosticsDir(root, "task"), filepath.Join(root, ".scafld", "runs", "task", "artifacts", "commands"); got != want {
		t.Fatalf("CommandDiagnosticsDir = %q, want %q", got, want)
	}
	if got, want := ProviderPacketsDir(root, "task"), filepath.Join(root, ".scafld", "runs", "task", "artifacts", "provider-packets"); got != want {
		t.Fatalf("ProviderPacketsDir = %q, want %q", got, want)
	}
	if got, want := SessionLockPath(root, "task"), filepath.Join(root, ".scafld", "locks", "runs", "task.lock"); got != want {
		t.Fatalf("SessionLockPath = %q, want %q", got, want)
	}
}

func TestSessionLockPathFromSessionPathMovesLocksOutOfRunDir(t *testing.T) {
	t.Parallel()

	sessionPath := filepath.Join("/repo", ".scafld", "runs", "task", "session.json")
	want := filepath.Join("/repo", ".scafld", "locks", "runs", "task.lock")
	if got := SessionLockPathFromSessionPath(sessionPath); got != want {
		t.Fatalf("lock path = %q, want %q", got, want)
	}

	nonstandard := filepath.Join("/repo", "custom", "session.json")
	if got, want := SessionLockPathFromSessionPath(nonstandard), nonstandard+".lock"; got != want {
		t.Fatalf("nonstandard lock path = %q, want %q", got, want)
	}
}
