package acceptance

import "fmt"

// ExpectedKind names the machine-checkable expectation for criterion evidence.
type ExpectedKind string

const (
	// ExpectedExitCodeZero passes when the command exits with code 0.
	ExpectedExitCodeZero ExpectedKind = "exit_code_zero"
	// ExpectedExitCodeNonzero passes when the command exits with a non-zero code.
	ExpectedExitCodeNonzero ExpectedKind = "exit_code_nonzero"
	// ExpectedNoMatches passes when the captured output is empty.
	ExpectedNoMatches ExpectedKind = "no_matches"
	// ExpectedManual marks a criterion that requires human evidence.
	ExpectedManual ExpectedKind = "manual"
	// ExpectedBrowserEvidence passes when a browser command exits cleanly and emits a valid browser evidence packet.
	ExpectedBrowserEvidence ExpectedKind = "browser_evidence"
)

// ValidExpectedKind reports whether kind is supported by the evaluator.
func ValidExpectedKind(kind ExpectedKind) bool {
	switch kind {
	case ExpectedExitCodeZero, ExpectedExitCodeNonzero, ExpectedNoMatches, ExpectedManual, ExpectedBrowserEvidence:
		return true
	default:
		return false
	}
}

// ExpectedKindValues returns the supported expected_kind values in display order.
func ExpectedKindValues() []ExpectedKind {
	return []ExpectedKind{ExpectedExitCodeZero, ExpectedExitCodeNonzero, ExpectedNoMatches, ExpectedManual, ExpectedBrowserEvidence}
}

// Evidence is the command output used to evaluate an acceptance criterion.
type Evidence struct {
	ExitCode   int
	Output     string
	Command    string
	Diagnostic string
}

// Result is the normalized pass/fail/pending outcome for criterion evidence.
type Result struct {
	Status string
	Reason string
}

// Evaluate compares evidence with the requested expected kind.
func Evaluate(kind ExpectedKind, evidence Evidence) Result {
	switch kind {
	case ExpectedExitCodeZero:
		if evidence.ExitCode == 0 {
			return Result{Status: "pass", Reason: "exit code was 0"}
		}
		return Result{Status: "fail", Reason: fmt.Sprintf("exit code was %d", evidence.ExitCode)}
	case ExpectedExitCodeNonzero:
		if evidence.ExitCode != 0 {
			return Result{Status: "pass", Reason: fmt.Sprintf("exit code was %d", evidence.ExitCode)}
		}
		return Result{Status: "fail", Reason: "exit code was 0"}
	case ExpectedNoMatches:
		if evidence.Output == "" {
			return Result{Status: "pass", Reason: "output was empty"}
		}
		return Result{Status: "fail", Reason: "output was not empty"}
	case ExpectedManual:
		return Result{Status: "pending", Reason: "manual criterion requires human evidence"}
	case ExpectedBrowserEvidence:
		return evaluateBrowserEvidence(evidence)
	default:
		return Result{Status: "invalid", Reason: "unknown expected_kind"}
	}
}
