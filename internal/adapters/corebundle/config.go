package corebundle

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
	"gopkg.in/yaml.v3"
)

func installProjectConfig(ctx context.Context, root string, result *Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := writeManagedFile(filepath.Join(root, ".scafld", "config.yaml"), ".scafld/config.yaml", projectConfig(), false, result); err != nil {
		return err
	}
	local := []byte(`# Local scafld overrides. This file should stay uncommitted.
#
# Keep this file to personal machine settings only. The full config shape lives
# in .scafld/core/config.yaml; the committed project config lives in
# .scafld/config.yaml.
#
# execution:
#   path_prepend:
#     - "$HOME/.rbenv/shims"
#     - "$HOME/.rbenv/bin"
# harden:
#   external:
#     provider: "codex"
#     codex:
#       model: "gpt-5.5"
# review:
#   external:
#     provider: "codex"
#     codex:
#       model: "gpt-5.5"
`)
	return writeManagedFile(filepath.Join(root, ".scafld", "config.local.yaml"), ".scafld/config.local.yaml", local, false, result)
}

func projectConfig() []byte {
	return []byte(`# scafld project configuration
#
# Keep this file sparse. Runtime defaults live in the scafld binary; the full
# example shape lives in .scafld/core/config.yaml.
#
# Put only project-specific invariant IDs, execution environment, review
# defaults, or review-pass overrides here. Use scafld config for
# evidence-backed suggestions before tightening this file.

version: "1.0"

invariants:
  canonical: {}

execution:
  path_prepend: []
  env: {}
`)
}

func normalizeProjectConfig(ctx context.Context, root string, result *Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rel := ".scafld/config.yaml"
	path := filepath.Join(root, filepath.FromSlash(rel))
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", rel, err)
	}
	normalized, changed, err := normalizeConfigYAML(data)
	if err != nil {
		return fmt.Errorf("normalize %s: %w", rel, err)
	}
	if !changed {
		result.Skipped = append(result.Skipped, rel)
		return nil
	}
	if err := atomicfile.Write(path, normalized, fileMode(rel)); err != nil {
		return fmt.Errorf("write %s: %w", rel, err)
	}
	result.Updated = append(result.Updated, rel)
	return nil
}

func normalizeConfigYAML(data []byte) ([]byte, bool, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, false, err
	}
	root := yamlDocumentRoot(&doc)
	canonical := yamlMappingLookup(yamlMappingLookup(root, "invariants"), "canonical")
	if canonical == nil || canonical.Kind != yaml.SequenceNode {
		return nil, false, nil
	}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, item := range canonical.Content {
		if item.Kind != yaml.ScalarNode || strings.TrimSpace(item.Value) == "" {
			return nil, false, fmt.Errorf("invariants.canonical list items must be strings")
		}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: strings.TrimSpace(item.Value)},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""},
		)
	}
	*canonical = *mapping
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(&doc); err != nil {
		_ = encoder.Close()
		return nil, false, err
	}
	if err := encoder.Close(); err != nil {
		return nil, false, err
	}
	return out.Bytes(), true, nil
}

func yamlDocumentRoot(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0]
	}
	return node
}

func yamlMappingLookup(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}
