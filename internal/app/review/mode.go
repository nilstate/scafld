package review

import (
	"context"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/review"
)

func reviewMode(ctx context.Context, sessions SessionStore, taskID string, input Input) review.Mode {
	if input.ForceMode {
		if input.Mode == review.ModeVerify {
			return review.ModeVerify
		}
		return review.ModeDiscover
	}
	switch strings.TrimSpace(input.RerunPolicy) {
	case "", "verify_open_blockers":
		if latestReviewHasOpenBlockers(ctx, sessions, taskID) {
			return review.ModeVerify
		}
	case "discover":
		return review.ModeDiscover
	case "verify":
		return review.ModeVerify
	}
	if input.Mode == review.ModeVerify {
		return review.ModeVerify
	}
	return review.ModeDiscover
}

func latestReviewHasOpenBlockers(ctx context.Context, sessions SessionStore, taskID string) bool {
	if sessions == nil {
		return false
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return false
	}
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		dossier, ok := review.DecodeDossier(entry.Output)
		return ok && review.OpenBlockerCount(dossier.Findings) > 0
	}
	return false
}

func reviewBudget(input Input, findings int, attacks int) review.Budget {
	return review.Budget{
		MaxFindings:        input.MaxFindings,
		MinAttackAngles:    input.MinAttackAngles,
		ActualFindings:     findings,
		ActualAttackAngles: attacks,
		Depth:              strings.TrimSpace(input.ReviewDepth),
	}
}

func applyRequestedBudget(dossier review.Dossier, requested review.Budget) review.Dossier {
	if dossier.Budget.MaxFindings == 0 {
		dossier.Budget.MaxFindings = requested.MaxFindings
	}
	if dossier.Budget.MinAttackAngles == 0 {
		dossier.Budget.MinAttackAngles = requested.MinAttackAngles
	}
	if strings.TrimSpace(dossier.Budget.Depth) == "" {
		dossier.Budget.Depth = requested.Depth
	}
	dossier.Budget.ActualFindings = len(dossier.Findings)
	dossier.Budget.ActualAttackAngles = len(dossier.AttackLog)
	return dossier
}
