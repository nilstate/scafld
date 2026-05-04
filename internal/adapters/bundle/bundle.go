package bundle

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

type Manifest struct {
	Hash string
}

func Verify(ctx context.Context, source string) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	h := sha256.New()
	err := filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = h.Write([]byte(path))
		_, _ = h.Write(data)
		return nil
	})
	if err != nil {
		return Manifest{}, err
	}
	return Manifest{Hash: fmt.Sprintf("%x", h.Sum(nil))}, nil
}
