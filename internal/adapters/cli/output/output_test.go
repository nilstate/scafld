package output

import (
	"strings"
	"testing"

	appreport "github.com/nilstate/scafld/v2/internal/app/report"
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
