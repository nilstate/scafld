package output

import (
	"strings"
	"testing"

	appreport "github.com/nilstate/scafld/v2/internal/app/report"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	appstatus "github.com/nilstate/scafld/v2/internal/app/status"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	corereview "github.com/nilstate/scafld/v2/internal/core/review"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

func TestReportPrintsReviewDossierMetrics(t *testing.T) {
	t.Parallel()

	text := Report(appreport.Output{
		Total: 1,
		Metrics: appreport.Metrics{
			SessionTotal:              1,
			FirstAttemptPassRate:      1,
			FirstAttemptPasses:        1,
			FirstAttemptTotal:         1,
			ReviewPassRate:            0.5,
			ReviewPasses:              1,
			ReviewTotal:               2,
			ReviewDossierCoverage:     1,
			ReviewDossierTotal:        2,
			ReviewFindingsTotal:       3,
			ReviewOpenBlockersTotal:   1,
			ReviewAttackAnglesTotal:   6,
			ReviewModeDistribution:    map[string]int{"discover": 1, "verify": 1},
			WorkspaceBaselineCoverage: 1,
			WorkspaceBaselineTasks:    1,
		},
	})
	for _, want := range []string{
		"- review_dossier_coverage: 100.0% (2/2)",
		"- review_findings_total: 3",
		"- review_open_blockers_total: 1",
		"- review_attack_angles_total: 6",
		"  - discover: 1",
		"  - verify: 1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("report output missing %q:\n%s", want, text)
		}
	}
}

func TestReportPrintsEmptyMetricGuidance(t *testing.T) {
	t.Parallel()

	text := Report(appreport.Output{})
	for _, want := range []string{"metrics:", "- first_attempt_pass_rate: n/a (0/0)", "- review_pass_rate: n/a (0/0)", "- workspace_baseline_coverage: n/a (0/0)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("empty report output missing %q:\n%s", want, text)
		}
	}
}

func TestReviewPrintsRepairContract(t *testing.T) {
	t.Parallel()

	text := Review(appreview.Output{
		TaskID:  "task",
		Verdict: corereview.VerdictFail,
		Findings: []corereview.Finding{{
			ID:               "bug",
			Severity:         corereview.SeverityHigh,
			BlocksCompletion: true,
			Summary:          "bug",
		}},
		Repair: &gate.Failure{
			Gate:     "review",
			Status:   "review",
			Reason:   "review verdict fail",
			Evidence: []string{"entry-1"},
			Expected: "review verdict pass",
			Actual:   "review verdict fail",
			Blockers: []string{"bug"},
			Next:     "scafld handoff task",
		},
	})
	for _, want := range []string{"gate: review", "status: review", "reason: review verdict fail", "evidence:", "expected: review verdict pass", "actual: review verdict fail", "blockers:", "next: scafld handoff task"} {
		if !strings.Contains(text, want) {
			t.Fatalf("review output missing %q:\n%s", want, text)
		}
	}
}

func TestStatusPrintsRepairContractBeforeStaleReviewVerdict(t *testing.T) {
	t.Parallel()

	text := Status(appstatus.Output{
		TaskID: "task",
		Status: spec.StatusReview,
		Next:   "scafld handoff task",
		Review: appstatus.ReviewInfo{
			Verdict: corereview.VerdictPass,
			Summary: "previous review passed",
		},
		Repair: &gate.Failure{
			Gate:     "review",
			Status:   "review",
			Reason:   "review provider failed",
			Evidence: []string{"/tmp/scafld-review.txt"},
			Expected: "valid ReviewDossier submitted by an external reviewer",
			Actual:   "provider produced no submission",
			Blockers: []string{"provider produced no submission"},
			Next:     "scafld handoff task",
		},
	})
	for _, want := range []string{"gate: review", "reason: review provider failed", "/tmp/scafld-review.txt", "expected: valid ReviewDossier submitted by an external reviewer", "actual: provider produced no submission", "blockers:", "review: pass"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q:\n%s", want, text)
		}
	}
}

func TestStatusPrintsCompletionAuthority(t *testing.T) {
	t.Parallel()

	text := Status(appstatus.Output{
		TaskID: "task",
		Status: spec.StatusCompleted,
		Next:   "none",
		Completion: &appstatus.CompletionInfo{
			Status:      "valid",
			Kind:        "human_reviewed",
			Provider:    "human",
			Verdict:     "pass",
			Summary:     "human-reviewed override: manual audit",
			ReviewEvent: "review-1",
		},
	})
	for _, want := range []string{"completion authority: valid (human_reviewed)", "authority review: pass by human", "authority summary: human-reviewed override: manual audit"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q:\n%s", want, text)
		}
	}
}

func TestStatusPrintsInvalidCompletionAuthority(t *testing.T) {
	t.Parallel()

	text := Status(appstatus.Output{
		TaskID: "task",
		Status: spec.StatusCompleted,
		Next:   "none",
		Completion: &appstatus.CompletionInfo{
			Status: "invalid",
			Kind:   "invalid",
			Reason: "latest review gate has not passed",
			Actual: "latest review verdict fail",
		},
	})
	for _, want := range []string{"completion authority: invalid (invalid)", "authority error: latest review gate has not passed", "authority actual: latest review verdict fail"} {
		if !strings.Contains(text, want) {
			t.Fatalf("status output missing %q:\n%s", want, text)
		}
	}
}
