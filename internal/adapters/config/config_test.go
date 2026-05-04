package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigLoad(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scafld"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Version != "1" {
		t.Fatalf("version = %q", cfg.Version)
	}
}
