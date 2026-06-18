package review

import (
	"strings"
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
)

func TestReviewRequestFramesBudgetsWithoutBrittleEnforcement(t *testing.T) {
	t.Parallel()

	body := reviewRequestBody(corereview.ModeDiscover, 4, 3, "standard", "")
	for _, want := range []string{
		"Max findings: 4",
		"Finding budget: report as many real defects",
		"do not spend slots on weak or speculative claims",
		"Minimum attack angles: 3",
		"record skipped angles instead of inventing findings",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("review request missing %q:\n%s", want, body)
		}
	}
}

func TestProviderInstructionMaximizesRealDefectsNotFalsePositives(t *testing.T) {
	t.Parallel()

	body := providerInstructionBody()
	for _, want := range []string{
		"Find as many real defects",
		"keep attacking after the first issue",
		"drop weak or speculative claims",
		"false positives",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("provider instruction missing %q:\n%s", want, body)
		}
	}
}
