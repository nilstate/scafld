//go:build windows

package filelock

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const lockfileExclusiveLock = 0x00000002

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

func lockFileEx(handle syscall.Handle, overlapped *syscall.Overlapped) error {
	r1, _, err := procLockFileEx.Call(
		uintptr(handle),
		uintptr(lockfileExclusiveLock),
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
