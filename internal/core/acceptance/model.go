package acceptance

import "fmt"

type ExpectedKind string

const (
	ExpectedExitCodeZero    ExpectedKind = "exit_code_zero"
	ExpectedExitCodeNonzero ExpectedKind = "exit_code_nonzero"
	ExpectedNoMatches       ExpectedKind = "no_matches"
	ExpectedManual          ExpectedKind = "manual"
)

func ValidExpectedKind(kind ExpectedKind) bool {
	switch kind {
	case ExpectedExitCodeZero, ExpectedExitCodeNonzero, ExpectedNoMatches, ExpectedManual:
		return true
	default:
		return false
	}
}

type Evidence struct {
	ExitCode int
	Output   string
}

type Result struct {
	Status string
	Reason string
}

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
	default:
		return Result{Status: "invalid", Reason: "unknown expected_kind"}
	}
}
