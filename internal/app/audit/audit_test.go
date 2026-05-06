package audit

import "testing"

func TestScopeReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	input := []string{"README.md", "internal/app/audit/audit.go"}
	got := Scope(input)
	if len(got) != len(input) {
		t.Fatalf("len = %d, want %d", len(got), len(input))
	}
	got[0] = "changed.md"
	if input[0] != "README.md" {
		t.Fatalf("scope result aliases input: input[0] = %q", input[0])
	}
}

func TestScopePreservesNil(t *testing.T) {
	t.Parallel()

	if got := Scope(nil); got != nil {
		t.Fatalf("Scope(nil) = %#v, want nil", got)
	}
}
