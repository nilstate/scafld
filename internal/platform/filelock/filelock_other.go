//go:build !unix

package filelock

func lock(path string) (func(), error) {
	return func() {}, nil
}
