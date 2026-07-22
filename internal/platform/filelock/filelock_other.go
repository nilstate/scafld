//go:build !unix && !windows

package filelock

import "context"

func lock(path string) (func(), error) {
	return func() {}, nil
}

func lockContext(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return lock(path)
}
