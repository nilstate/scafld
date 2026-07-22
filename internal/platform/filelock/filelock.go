// Package filelock provides advisory cross-process file locking for stores
// that read-modify-write shared state. In-process callers must still serialize
// with their own mutex; the file lock only fences other processes.
package filelock

import "context"

// Lock acquires an exclusive advisory lock on path, blocking until the lock
// is available. It returns a release function that must be called to unlock.
// On platforms without a supported scafld install channel it is a no-op,
// preserving the previous in-process-only guarantee rather than failing.
func Lock(path string) (func(), error) {
	return lock(path)
}

// LockContext acquires an exclusive advisory lock on path while honoring ctx.
func LockContext(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return lockContext(ctx, path)
}
