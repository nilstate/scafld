package harden

import (
	"strings"
	"testing"
)

func TestProviderInstructionChallengesRightToExistWithoutFiller(t *testing.T) {
	t.Parallel()

	body := hardenProviderInstructionBody()
	for _, want := range []string{
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
