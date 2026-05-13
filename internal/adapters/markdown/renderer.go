package markdown

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/acceptance"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// Render converts a normalized model into canonical living Markdown.
func Render(model spec.Model) []byte {
	var b strings.Builder
	writeFrontMatter(&b, model)
	fmt.Fprintf(&b, "# %s\n\n", fallback(model.Title, model.TaskID))
	fmt.Fprintf(&b, "## Current State\n\n")
	fmt.Fprintf(&b, "Status: %s\n", fallback(string(model.Status), "draft"))
	fmt.Fprintf(&b, "Current phase: %s\n", fallback(model.CurrentState.CurrentPhase, "none"))
	fmt.Fprintf(&b, "Next: %s\n", fallback(model.CurrentState.Next, "approve"))
	fmt.Fprintf(&b, "Reason: %s\n", fallback(model.CurrentState.Reason, "new task spec"))
	fmt.Fprintf(&b, "Blockers: %s\n", fallback(model.CurrentState.Blockers, "none"))
	fmt.Fprintf(&b, "Allowed follow-up command: `%s`\n", fallback(model.CurrentState.AllowedFollowUp, "scafld status "+model.TaskID))
	fmt.Fprintf(&b, "Latest runner update: %s\n", fallback(model.CurrentState.LatestRunnerUpdate, "none"))
	fmt.Fprintf(&b, "Review gate: %s\n\n", fallback(model.CurrentState.ReviewGate, "not_started"))
	fmt.Fprintf(&b, "## Summary\n\n%s\n\n", fallback(model.Summary, "No summary yet."))
	renderContext(&b, model.Context)
	renderStringList(&b, "Objectives", model.Objectives)
	renderStringList(&b, "Scope", model.Scope)
	renderStringList(&b, "Dependencies", model.Dependencies)
	renderStringList(&b, "Assumptions", model.Assumptions)
	renderStringList(&b, "Touchpoints", model.Touchpoints)
	renderRisks(&b, model.Risks)
	fmt.Fprintf(&b, "## Acceptance\n\nProfile: %s\n\nValidation:\n", fallback(model.Acceptance.ValidationProfile, "standard"))
	renderCriteria(&b, model.Acceptance.Criteria)
	if len(model.Acceptance.Criteria) == 0 {
		fmt.Fprintf(&b, "- none\n")
	}
	fmt.Fprintf(&b, "\n")
	for _, phase := range model.Phases {
		number := phase.Number
		if number == 0 {
			number = len(model.Phases)
		}
		fmt.Fprintf(&b, "## Phase %d: %s\n\n", number, fallback(phase.Name, phase.ID))
		fmt.Fprintf(&b, "Status: %s\n", fallback(phase.Status, "pending"))
		fmt.Fprintf(&b, "Dependencies: %s\n\n", fallback(strings.Join(phase.Dependencies, ", "), "none"))
		fmt.Fprintf(&b, "Objective: %s\n\n", fallback(phase.Objective, "Complete this phase."))
		fmt.Fprintf(&b, "Changes:\n")
		renderBullets(&b, phase.Changes)
		fmt.Fprintf(&b, "\nAcceptance:\n")
		renderCriteria(&b, phase.Acceptance)
		if len(phase.Acceptance) == 0 {
			fmt.Fprintf(&b, "- none\n")
		}
		fmt.Fprintf(&b, "\n")
	}
	renderStringList(&b, "Rollback", model.Rollback)
	renderReview(&b, model.Review)
	renderStringList(&b, "Self Eval", model.SelfEval)
	renderStringList(&b, "Deviations", model.Deviations)
	fmt.Fprintf(&b, "## Metadata\n\n")
	if len(model.Metadata) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		keys := make([]string, 0, len(model.Metadata))
		for key := range model.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "- %s: %s\n", key, model.Metadata[key])
		}
	}
	fmt.Fprintf(&b, "\n## Origin\n\nCreated by: %s\nSource: %s\n\n", fallback(model.Origin.CreatedBy, "scafld"), fallback(model.Origin.Source, "plan"))
	fmt.Fprintf(&b, "## Harden Rounds\n\n")
	if len(model.HardenRounds) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		for _, round := range model.HardenRounds {
			renderHardenRound(&b, round)
		}
	}
	fmt.Fprintf(&b, "\n## Planning Log\n\n")
	if len(model.PlanningLog) == 0 {
		fmt.Fprintf(&b, "- none\n")
	} else {
		for _, event := range model.PlanningLog {
			fmt.Fprintf(&b, "- %s %s\n", event.Time, event.Text)
		}
	}
	return []byte(b.String())
}

func writeFrontMatter(b *strings.Builder, model spec.Model) {
	fmt.Fprintf(b, "---\n")
	fmt.Fprintf(b, "spec_version: '%s'\n", fallback(model.Version, "2.0"))
	fmt.Fprintf(b, "task_id: %s\n", model.TaskID)
	fmt.Fprintf(b, "created: '%s'\n", model.Created)
	fmt.Fprintf(b, "updated: '%s'\n", model.Updated)
	fmt.Fprintf(b, "status: %s\n", fallback(string(model.Status), "draft"))
	fmt.Fprintf(b, "harden_status: %s\n", fallback(string(model.HardenStatus), "not_run"))
	fmt.Fprintf(b, "size: %s\n", fallback(string(model.Size), "medium"))
	fmt.Fprintf(b, "risk_level: %s\n", fallback(string(model.RiskLevel), "medium"))
	fmt.Fprintf(b, "---\n\n")
}

func renderStringList(b *strings.Builder, title string, items []string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	renderBullets(b, items)
	fmt.Fprintf(b, "\n")
}

func renderContext(b *strings.Builder, context spec.Context) {
	if context.CWD == "" && len(context.Packages) == 0 && len(context.FilesImpacted) == 0 && len(context.Invariants) == 0 && len(context.RelatedDocs) == 0 {
		return
	}
	fmt.Fprintf(b, "## Context\n\n")
	if context.CWD != "" {
		fmt.Fprintf(b, "CWD: `%s`\n\n", context.CWD)
	}
	renderBullets(b, context.Packages)
	fmt.Fprintf(b, "\n")
}

func renderRisks(b *strings.Builder, risks []spec.Risk) {
	fmt.Fprintf(b, "## Risks\n\n")
	if len(risks) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, risk := range risks {
		fmt.Fprintf(b, "- %s", risk.Description)
		if risk.Mitigation != "" {
			fmt.Fprintf(b, " - %s", risk.Mitigation)
		}
		fmt.Fprintf(b, "\n")
	}
	fmt.Fprintf(b, "\n")
}

func renderReview(b *strings.Builder, review spec.ReviewState) {
	fmt.Fprintf(b, "## Review\n\nStatus: %s\nVerdict: %s\n", fallback(review.Status, "not_started"), fallback(review.Verdict, "none"))
	if review.Mode != "" {
		fmt.Fprintf(b, "Mode: %s\n", review.Mode)
	}
	if review.Provider != "" {
		if review.Model != "" {
			fmt.Fprintf(b, "Provider: %s:%s\n", review.Provider, review.Model)
		} else {
			fmt.Fprintf(b, "Provider: %s\n", review.Provider)
		}
	}
	if review.OutputFormat != "" {
		fmt.Fprintf(b, "Output: %s\n", review.OutputFormat)
	}
	if len(review.Normalizations) > 0 {
		fmt.Fprintf(b, "Normalizations: %s\n", strings.Join(review.Normalizations, ", "))
	}
	if review.Summary != "" {
		fmt.Fprintf(b, "Summary: %s\n", review.Summary)
	}
	if len(review.AttackLog) > 0 {
		fmt.Fprintf(b, "\nAttack log:\n")
		for _, attack := range review.AttackLog {
			fmt.Fprintf(b, "- `%s`: %s -> %s", fallback(attack.Target, "target"), fallback(attack.Attack, "attack"), fallback(string(attack.Result), "result"))
			if attack.Notes != "" {
				fmt.Fprintf(b, " (%s)", attack.Notes)
			}
			fmt.Fprintf(b, "\n")
		}
	}
	fmt.Fprintf(b, "\nFindings:\n")
	if len(review.Findings) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, finding := range review.Findings {
		blocking := "non-blocking"
		if corereview.BlocksCompletion(finding) {
			blocking = "blocks completion"
		}
		fmt.Fprintf(b, "- [%s/%s] `%s` %s\n", fallback(string(finding.Severity), string(corereview.SeverityLow)), blocking, fallback(finding.ID, "finding"), fallback(finding.Summary, "No summary recorded."))
		if finding.Location != nil && finding.Location.Path != "" {
			if finding.Location.Line > 0 {
				fmt.Fprintf(b, "  - Location: `%s:%d`\n", finding.Location.Path, finding.Location.Line)
			} else {
				fmt.Fprintf(b, "  - Location: `%s`\n", finding.Location.Path)
			}
		}
		if finding.Evidence != "" {
			fmt.Fprintf(b, "  - Evidence: %s\n", finding.Evidence)
		}
		if finding.Impact != "" {
			fmt.Fprintf(b, "  - Impact: %s\n", finding.Impact)
		}
		if finding.Validation != "" {
			fmt.Fprintf(b, "  - Validation: %s\n", finding.Validation)
		}
	}
	fmt.Fprintf(b, "\n")
}

func renderHardenRound(b *strings.Builder, round spec.HardenRound) {
	fmt.Fprintf(b, "### %s\n\n", fallback(round.ID, "round"))
	fmt.Fprintf(b, "Status: %s\n", fallback(round.Status, "in_progress"))
	fmt.Fprintf(b, "Started: %s\n", fallback(round.StartedAt, "none"))
	fmt.Fprintf(b, "Ended: %s\n\n", fallback(round.EndedAt, "none"))
	fmt.Fprintf(b, "Checks:\n")
	if len(round.Checks) == 0 {
		fmt.Fprintf(b, "- none\n\n")
	} else {
		for _, check := range round.Checks {
			fmt.Fprintf(b, "- %s\n", fallback(check.Name, "Check not recorded."))
			if check.GroundedIn != "" {
				fmt.Fprintf(b, "  - Grounded in: %s\n", check.GroundedIn)
			}
			if check.Result != "" {
				fmt.Fprintf(b, "  - Result: %s\n", check.Result)
			}
			if check.Evidence != "" {
				fmt.Fprintf(b, "  - Evidence: %s\n", check.Evidence)
			}
		}
		fmt.Fprintf(b, "\n")
	}
	fmt.Fprintf(b, "Questions:\n")
	if len(round.Questions) == 0 {
		fmt.Fprintf(b, "- none\n\n")
		return
	}
	for _, question := range round.Questions {
		fmt.Fprintf(b, "- %s\n", fallback(question.Question, "Question not recorded."))
		if question.GroundedIn != "" {
			fmt.Fprintf(b, "  - Grounded in: %s\n", question.GroundedIn)
		}
		if question.RecommendedAnswer != "" {
			fmt.Fprintf(b, "  - Recommended answer: %s\n", question.RecommendedAnswer)
		}
		if question.IfUnanswered != "" {
			fmt.Fprintf(b, "  - If unanswered: %s\n", question.IfUnanswered)
		}
		if question.AnsweredWith != "" {
			fmt.Fprintf(b, "  - Answered with: %s\n", question.AnsweredWith)
		}
	}
	fmt.Fprintf(b, "\n")
}

func renderBullets(b *strings.Builder, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "- none\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func renderCriteria(b *strings.Builder, criteria []spec.Criterion) {
	for _, c := range criteria {
		fmt.Fprintf(b, "- [%s] `%s` %s - %s\n", checked(c.Status), c.ID, fallback(c.Type, "command"), fallback(c.Title, c.Command))
		if c.Command != "" {
			fmt.Fprintf(b, "  - Command: `%s`\n", c.Command)
		}
		fmt.Fprintf(b, "  - Expected kind: `%s`\n", fallback(string(c.ExpectedKind), string(acceptance.ExpectedExitCodeZero)))
		fmt.Fprintf(b, "  - Status: %s\n", fallback(c.Status, "pending"))
		if c.Evidence != "" {
			fmt.Fprintf(b, "  - Evidence: %s\n", c.Evidence)
		}
		if c.SourceEvent != "" {
			fmt.Fprintf(b, "  - Source event: %s\n", c.SourceEvent)
		}
	}
}

func checked(status string) string {
	if status == "pass" || status == "completed" {
		return "x"
	}
	return " "
}

func fallback(value, fb string) string {
	if strings.TrimSpace(value) == "" {
		return fb
	}
	return value
}
