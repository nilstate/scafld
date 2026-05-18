package harden

import (
	"context"
	"fmt"
	"sort"
	"strings"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	promptadapter "github.com/nilstate/scafld/v2/internal/adapters/prompts"
)

// Prompt returns the hardening prompt plus relevant workspace configuration context.
func Prompt(ctx context.Context, root string) string {
	prompt := promptadapter.LoadHarden(root)
	cfg, err := configadapter.Load(ctx, root)
	if err != nil || cfg.Harden.MaxIssuesPerRound <= 0 {
		return prompt
	}
	var b strings.Builder
	b.WriteString(prompt)
	fmt.Fprintf(&b, "\n\nConfigured max_issues_per_round: %d. This is a cap, not a target.\n", cfg.Harden.MaxIssuesPerRound)
	if len(cfg.Invariants.Canonical) > 0 {
		b.WriteString("\nConfigured project invariants:\n")
		keys := make([]string, 0, len(cfg.Invariants.Canonical))
		for key := range cfg.Invariants.Canonical {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			description := strings.TrimSpace(cfg.Invariants.Canonical[key])
			if description == "" {
				fmt.Fprintf(&b, "- %s\n", key)
			} else {
				fmt.Fprintf(&b, "- %s: %s\n", key, description)
			}
		}
	}
	return b.String()
}
