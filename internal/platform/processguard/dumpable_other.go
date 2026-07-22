//go:build !linux

package processguard

// DisableDumpable is a Linux procfs hardening hook. Other platforms either do
// not expose /proc/$PPID/environ in this form or need a different control.
func DisableDumpable() (func(), error) {
	return func() {}, nil
}
