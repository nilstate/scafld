//go:build !unix && !windows

package filelock

func lock(path string) (func(), error) {
	return func() {}, nil
}
