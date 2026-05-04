package acceptance

import "testing"

func TestExpectedKindInvalidEvidence(t *testing.T) {
	t.Parallel()

	if !ValidExpectedKind(ExpectedExitCodeZero) {
		t.Fatal("expected kind should be valid")
	}
	if ValidExpectedKind("invalid") {
		t.Fatal("invalid kind accepted")
	}
	if got := Evaluate(ExpectedExitCodeZero, Evidence{ExitCode: 0}); got.Status != "pass" {
		t.Fatalf("zero exit result = %+v", got)
	}
	if got := Evaluate(ExpectedExitCodeZero, Evidence{ExitCode: 7}); got.Status != "fail" {
		t.Fatalf("nonzero exit result = %+v", got)
	}
}
