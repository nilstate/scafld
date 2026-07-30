package review

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	corereview "github.com/nilstate/scafld/v2/internal/core/review"
)

func TestReviewPromptPolicyCarriesShapeAndFindingDiscipline(t *testing.T) {
	t.Parallel()

	embedded, err := os.ReadFile(filepath.Join("..", "..", "adapters", "corebundle", "assets", "core", "prompts", "review.md"))
	if err != nil {
		t.Fatal(err)
	}
	mustContain(t, reviewRequestBody(corereview.ModeDiscover, 4, 3, "standard", ""), "Max findings: 4", "Finding budget: report as many real defects", "do not spend slots on weak or speculative claims", "Minimum attack angles: 3", "record skipped angles instead of inventing findings")
	mustContain(t, providerInstructionBody(), "final-shape drift", "task-scoped product edges conflict", "return a finding", "suggested_fix", "drop weak or speculative claims", "false positives")
	mustContain(t, reviewOutputContractSection().Body, "Completion-blocking findings require `location`, `evidence`, `impact`, `suggested_fix`, and `validation`", "Final-shape drift findings are repair packets", "tell the executor exactly what to clean up")
	mustContain(t, string(embedded), "**Final product shape**", "Final-shape drift is a defect when verified", "return that as a finding for the executor to repair", "`findings[].suggested_fix`")
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Fatalf("prompt policy missing %q:\n%s", want, body)
		}
	}
}
