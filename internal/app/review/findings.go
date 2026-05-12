package review

import (
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/session"
	"github.com/nilstate/scafld/v2/internal/core/spec"
	coreworkspace "github.com/nilstate/scafld/v2/internal/core/workspace"
)

func workspaceMutationFinding(mutated []string) review.Finding {
	path := "."
	if len(mutated) > 0 {
		path = coreworkspace.ParseChange(mutated[0]).Path
		if path == "" {
			path = strings.TrimSpace(mutated[0])
		}
	}
	return review.Finding{
		ID:               "workspace_mutation",
		Severity:         review.SeverityCritical,
		BlocksCompletion: true,
		Category:         "review_integrity",
		Confidence:       review.ConfidenceHigh,
		Location:         &review.Location{Path: path},
		Evidence:         "workspace changed during review: " + strings.Join(mutated, ", "),
		Impact:           "The review provider changed the workspace while acting as a read-only reviewer, so its verdict is not trustworthy.",
		Validation:       "Restore the workspace to the expected state, ensure the provider is read-only, then rerun scafld review.",
		Summary:          "Workspace changed during review.",
	}
}

func appendSummary(current string, extra string) string {
	current = strings.TrimSpace(current)
	extra = strings.TrimSpace(extra)
	if current == "" {
		return extra
	}
	if extra == "" {
		return current
	}
	return current + " " + extra
}

func nextForVerdict(taskID string, verdict string) (string, string) {
	if verdict == "pass" {
		return "complete", "scafld complete " + taskID
	}
	return "repair", "scafld handoff " + taskID
}

func reviewRepair(model spec.Model, dossier review.Dossier, command string, evidence string) *gate.Failure {
	if dossier.Verdict != review.VerdictFail {
		return nil
	}
	blockers := make([]string, 0, len(dossier.Findings))
	for _, finding := range dossier.Findings {
		if !review.BlocksCompletion(finding) {
			continue
		}
		label := finding.ID
		if finding.Summary != "" {
			label += ": " + finding.Summary
		}
		if finding.Validation != "" {
			label += " | validate: " + finding.Validation
		}
		blockers = append(blockers, label)
	}
	return &gate.Failure{
		Gate:     "review",
		Status:   string(model.Status),
		Reason:   "review verdict fail",
		Evidence: []string{evidence},
		Expected: "review verdict pass",
		Actual:   "review verdict fail",
		Blockers: blockers,
		Next:     command,
	}
}

func latestReviewEvidence(ledger session.Session) string {
	for i := len(ledger.Entries) - 1; i >= 0; i-- {
		entry := ledger.Entries[i]
		if entry.Type != "review" {
			continue
		}
		if entry.ID != "" {
			return entry.ID
		}
		return "review event"
	}
	return "review event"
}
