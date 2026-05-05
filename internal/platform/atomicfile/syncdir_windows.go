package atomicfile

// syncDir is a no-op on Windows where directory fsync is not supported.
func syncDir(dir string) error {
	return nil
}
