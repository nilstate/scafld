//go:build unix

package filelock

import (
	"fmt"
	"os"
	"syscall"
)

func lock(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() {
		// Closing the descriptor releases the flock; the lock file is left in
		// place so concurrent acquirers never race a delete.
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
