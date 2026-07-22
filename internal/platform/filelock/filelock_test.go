package filelock

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestLockContextReturnsWhenContended(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "plan9" || runtime.GOOS == "js" || runtime.GOOS == "wasip1" {
		t.Skip("platform file locks are no-op on this target")
	}

	path := filepath.Join(t.TempDir(), "session.lock")
	release, err := Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	lateRelease, err := LockContext(ctx, path)
	if lateRelease != nil {
		lateRelease()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("LockContext error = %v, want deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("LockContext did not return promptly: %s", elapsed)
	}
}
