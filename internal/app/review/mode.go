package review

import (
	"context"
	"fmt"
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
	dossier, ok := latestReviewDossier(ctx, sessions, taskID)
	return ok && review.OpenBlockerCount(dossier.Findings) > 0
}

func latestReviewDossier(ctx context.Context, sessions SessionStore, taskID string) (review.Dossier, bool) {
	if sessions == nil {
		return review.Dossier{}, false
	}
	ledger, err := sessions.Load(ctx, taskID)
	if err != nil {
		return review.Dossier{}, false
	}
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		return review.DecodeDossier(entry.Output)
	}
	return review.Dossier{}, false
}

func knownFindingsForMode(ctx context.Context, sessions SessionStore, taskID string, mode review.Mode) []review.Finding {
	if mode != review.ModeVerify {
		return nil
	}
	dossier, ok := latestReviewDossier(ctx, sessions, taskID)
	if !ok {
		return nil
	}
	findings := make([]review.Finding, 0, len(dossier.Findings))
	for _, finding := range dossier.Findings {
		if review.BlocksCompletion(finding) {
			findings = append(findings, finding)
		}
	}
	return findings
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

func validateRequestedBudget(dossier review.Dossier) error {
	if dossier.Budget.MinAttackAngles > 0 && len(dossier.AttackLog) < dossier.Budget.MinAttackAngles {
		return fmt.Errorf("%w: attack_log has %d entries, requested at least %d", review.ErrInvalidDossier, len(dossier.AttackLog), dossier.Budget.MinAttackAngles)
	}
	if dossier.Budget.MaxFindings > 0 && len(dossier.Findings) > dossier.Budget.MaxFindings {
		return fmt.Errorf("%w: findings has %d entries, requested at most %d", review.ErrInvalidDossier, len(dossier.Findings), dossier.Budget.MaxFindings)
	}
	return nil
}
