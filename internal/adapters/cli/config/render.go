package config

import (
	"context"
	"fmt"
	"path/filepath"

	appconfig "github.com/nilstate/scafld/v2/internal/app/config"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
	"gopkg.in/yaml.v3"
)

// Run builds and writes the CLI config proposal.
func Run(ctx context.Context, root string) (appconfig.Output, error) {
	out, err := appconfig.Run(ctx, Scanner{Root: root})
	if err != nil {
		return appconfig.Output{}, err
	}
	out.Path = ".scafld/config.proposed.yaml"
	data, err := RenderProposal(out.Proposal)
	if err != nil {
		return appconfig.Output{}, err
	}
	if err := atomicfile.Write(filepath.Join(root, ".scafld", "config.proposed.yaml"), data, 0o644); err != nil {
		return appconfig.Output{}, fmt.Errorf("write config proposal: %w", err)
	}
	return out, nil
}

// RenderProposal serializes a config proposal for operator review.
func RenderProposal(proposal appconfig.Proposal) ([]byte, error) {
	data, err := yaml.Marshal(proposal)
	if err != nil {
		return nil, fmt.Errorf("marshal config proposal: %w", err)
	}
	return append(data, '\n'), nil
}
