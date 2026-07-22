package harden

import (
	"strings"
	"testing"
)

func TestProviderInstructionChallengesRightToExistWithoutFiller(t *testing.T) {
	t.Parallel()

	body := hardenProviderInstructionBody()
	for _, want := range []string{
		"canonical task input under review",
		"not evidence that the proposed shape",
		"reject/no-op",
		"reuse-existing-behavior",
		"materially better shape",
		"report shrink or reframe",
		"code-shape and system-design gate",
		"not coverage bookkeeping",
		"consumer-by-consumer compliance matrix",
		"bespoke test for every surface",
		"Per-surface implementation correctness belongs to build evidence and review",
		"right to exist",
		"as many real spec issues",
		"do not pad the dossier",
		"weak or speculative observations",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("provider instruction missing %q:\n%s", want, body)
		}
	}
}
