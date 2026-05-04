package harden

import (
	"context"
	"fmt"

	configadapter "github.com/nilstate/scafld/v2/internal/adapters/config"
	promptadapter "github.com/nilstate/scafld/v2/internal/adapters/prompts"
)

// Prompt returns the hardening prompt plus relevant workspace configuration context.
func Prompt(ctx context.Context, root string) string {
	prompt := promptadapter.LoadHarden(root)
	cfg, err := configadapter.Load(ctx, root)
	if err != nil || cfg.Harden.MaxQuestionsPerRound <= 0 {
		return prompt
	}
	return prompt + fmt.Sprintf("\n\nConfigured max_questions_per_round: %d. This is a cap, not a target.\n", cfg.Harden.MaxQuestionsPerRound)
}
