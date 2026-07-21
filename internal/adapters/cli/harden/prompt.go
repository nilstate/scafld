package harden

import (
	"context"
	"fmt"
	"sort"
	"strings"

	contractloader "github.com/nilstate/scafld/v2/internal/adapters/cli/agentcontract"
	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	corecontract "github.com/nilstate/scafld/v2/internal/core/agentcontract"
)

// Contract returns the hardening contract plus relevant workspace configuration context.
func Contract(ctx context.Context, root string) (corecontract.Contract, error) {
	contract, err := contractloader.Load(ctx, root, corecontract.RoleHarden)
	if err != nil {
		return corecontract.Contract{}, err
	}
	cfg, err := configadapter.Load(ctx, root)
	if err != nil || cfg.Harden.MaxIssuesPerRound <= 0 {
		return contract, nil
	}
	var b strings.Builder
	b.WriteString(contract.Body)
	fmt.Fprintf(&b, "\n\nConfigured max_issues_per_round: %d. This is a budget for real findings, not filler.\n", cfg.Harden.MaxIssuesPerRound)
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
	return corecontract.New(corecontract.RoleHarden, contract.Path, []byte(b.String()))
}

// Prompt returns the manual hardening prompt. Provider hardening receives the
// same shared contract with a provider-specific output contract instead.
func Prompt(ctx context.Context, root string) string {
	contract, err := Contract(ctx, root)
	if err != nil {
		return ""
	}
	return contract.Body + "\n\n## Manual Harden Output Contract\n\nFill the generated shape decision fields and observation rows under the latest harden round in the spec. Keep harden_status in_progress until the operator runs --mark-passed. Do not modify code while hardening.\n"
}
