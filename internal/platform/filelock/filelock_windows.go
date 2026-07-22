//go:build windows

package filelock

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	lockPollInterval        = 25 * time.Millisecond
)

const (
	errorSharingViolation syscall.Errno = 32
	errorLockViolation    syscall.Errno = 33
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx        = kernel32.NewProc("LockFileEx")
	procUnlockFileEx      = kernel32.NewProc("UnlockFileEx")
	errLockFileExFailed   = syscall.Errno(0)
	errUnlockFileExFailed = syscall.Errno(0)
)

func lock(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	var overlapped syscall.Overlapped
	if err := lockFileEx(syscall.Handle(file.Fd()), &overlapped); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() {
		_ = unlockFileEx(syscall.Handle(file.Fd()), &overlapped)
		_ = file.Close()
	}, nil
}

func lockContext(ctx context.Context, path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	var overlapped syscall.Overlapped
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		err := lockFileExWithFlags(syscall.Handle(file.Fd()), &overlapped, lockfileExclusiveLock|lockfileFailImmediately)
		if err == nil {
			return func() {
				_ = unlockFileEx(syscall.Handle(file.Fd()), &overlapped)
				_ = file.Close()
			}, nil
		}
		if !isLockUnavailable(err) {
			_ = file.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		timer := time.NewTimer(lockPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			_ = file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func lockFileEx(handle syscall.Handle, overlapped *syscall.Overlapped) error {
	return lockFileExWithFlags(handle, overlapped, lockfileExclusiveLock)
}

func lockFileExWithFlags(handle syscall.Handle, overlapped *syscall.Overlapped, flags uint32) error {
	r1, _, err := procLockFileEx.Call(
		uintptr(handle),
		uintptr(flags),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 == 0 {
		if err != errLockFileExFailed {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

func isLockUnavailable(err error) bool {
	errno, ok := err.(syscall.Errno)
	return ok && (errno == errorLockViolation || errno == errorSharingViolation)
}

func unlockFileEx(handle syscall.Handle, overlapped *syscall.Overlapped) error {
	r1, _, err := procUnlockFileEx.Call(
		uintptr(handle),
		0,
		uintptr(^uint32(0)),
		uintptr(^uint32(0)),
		uintptr(unsafe.Pointer(overlapped)),
	)
	if r1 == 0 {
		if err != errUnlockFileExFailed {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}
