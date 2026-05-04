package config

import (
	"context"
	"os"
	"path/filepath"
)

type Config struct {
	Version string
}

func Load(ctx context.Context, root string) (Config, error) {
	if err := ctx.Err(); err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(filepath.Join(root, ".scafld", "config.yaml"))
	if err != nil {
		return Config{}, err
	}
	if len(data) == 0 {
		return Config{}, nil
	}
	return Config{Version: "1"}, nil
}
