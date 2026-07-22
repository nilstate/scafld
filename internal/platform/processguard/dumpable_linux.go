//go:build linux

package processguard

import (
	"fmt"
	"syscall"
)

const (
	prGetDumpable = 3
	prSetDumpable = 4
)

// DisableDumpable prevents same-uid child processes from reading this process
// through ptrace-gated procfs files such as /proc/$PPID/environ.
func DisableDumpable() (func(), error) {
	original, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prGetDumpable, 0, 0, 0, 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("read process dumpability: %w", errno)
	}
	if _, _, errno := syscall.Syscall6(syscall.SYS_PRCTL, prSetDumpable, 0, 0, 0, 0, 0); errno != 0 {
		return nil, fmt.Errorf("disable process dumpability: %w", errno)
	}
	return func() {
		_, _, _ = syscall.Syscall6(syscall.SYS_PRCTL, prSetDumpable, original, 0, 0, 0, 0)
	}, nil
}
