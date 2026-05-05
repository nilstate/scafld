package handoff

import (
	"context"
	"fmt"
	"strings"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// SpecStore is the spec persistence port used by handoff rendering.
type SpecStore interface {
	Load(context.Context, string) (spec.Model, string, error)
}

// SessionStore loads session evidence for repair handoffs.
type SessionStore interface {
	Load(context.Context, string) (session.Session, error)
}

// Run renders the model-facing handoff for taskID.
func Run(ctx context.Context, specs SpecStore, sessions SessionStore, taskID string) (string, error) {
	model, _, err := specs.Load(ctx, taskID)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Handoff: %s\n\nStatus: %s\nNext: %s\n", model.Title, model.Status, model.CurrentState.AllowedFollowUp)
	if sessions != nil {
		if ledger, err := sessions.Load(ctx, model.TaskID); err == nil {
			writeLatestReviewFindings(&b, ledger)
		}
	}
	return b.String(), nil
}

func writeLatestReviewFindings(b *strings.Builder, ledger session.Session) {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		findings := corereview.DecodeFindings(entry.Output)
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(b, "\n## Review Findings\n\nVerdict: %s\n\n", entry.Status)
		for _, finding := range findings {
			fmt.Fprintf(b, "- [%s] %s: %s\n", finding.Severity, finding.ID, finding.Summary)
		}
		return
	}
}
