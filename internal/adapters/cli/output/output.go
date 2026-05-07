// Package output formats human-readable command output for the CLI adapter.
package output

import (
	"errors"
	"fmt"
	"strings"

	appbuild "github.com/nilstate/scafld/v2/internal/app/build"
	appreport "github.com/nilstate/scafld/v2/internal/app/report"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	appstatus "github.com/nilstate/scafld/v2/internal/app/status"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// CodeName maps process exit codes to stable JSON error codes.
func CodeName(exit int) string {
	switch exit {
	case 2:
		return "invalid_input"
	case 3:
		return "validation_failed"
	case 4:
		return "review_failed"
	case 5:
		return "cancelled"
	case 6:
		return "workspace_error"
	default:
		return "runtime_error"
	}
}

// GateFailure extracts a deterministic gate payload from err when present.
func GateFailure(err error) *gate.Failure {
	var gateErr gate.Error
	if !errors.As(err, &gateErr) {
		return nil
	}
	failure := gateErr.Failure
	return &failure
}

// GateFailureFromResult extracts repair payloads from non-zero successful use-case outputs.
func GateFailureFromResult(result any) *gate.Failure {
	withGate, ok := result.(interface{ GateFailure() *gate.Failure })
	if !ok {
		return nil
	}
	return withGate.GateFailure()
}

// Build formats build evidence and repair details when acceptance blocks.
func Build(out appbuild.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "build %s: %d passed, %d failed\n", out.Status, out.Passed, out.Failed)
	if out.Repair != nil {
		fmt.Fprintf(&b, "expected: %s\nactual: %s\n", out.Repair.Expected, out.Repair.Actual)
		for _, blocker := range out.Repair.Blockers {
			fmt.Fprintf(&b, "- %s\n", blocker)
		}
	}
	if out.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", out.Next)
	}
	return b.String()
}

// Review formats the review gate result so findings are visible in the normal path.
func Review(out appreview.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "review verdict: %s\n", out.Verdict)
	if len(out.Findings) > 0 {
		fmt.Fprintf(&b, "findings:\n")
		for _, finding := range out.Findings {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", finding.Severity, finding.ID, finding.Summary)
		}
	}
	if out.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", out.Next)
	}
	return b.String()
}

// Status formats status output with the latest review findings when present.
func Status(out appstatus.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\nnext: %s\n", out.TaskID, out.Status, out.Next)
	if out.Review.Running {
		fmt.Fprintf(&b, "review: running\n")
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	} else if out.Review.Verdict != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.Verdict)
		for _, finding := range out.Review.Findings {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", finding.Severity, finding.ID, finding.Summary)
		}
	} else if out.Review.AttemptStatus != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.AttemptStatus)
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	}
	return b.String()
}

// Report formats workspace reporting metrics.
func Report(out appreport.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "total specs: %d\n", out.Total)
	if len(out.ByStatus) > 0 {
		fmt.Fprintf(&b, "by status:\n")
		for _, status := range []string{"draft", "approved", "active", "blocked", "review", "completed", "failed", "cancelled"} {
			count := out.ByStatus[spec.Status(status)]
			if count > 0 {
				fmt.Fprintf(&b, "- %s: %d\n", status, count)
			}
		}
	}
	m := out.Metrics
	if m.SessionTotal > 0 {
		fmt.Fprintf(&b, "metrics:\n")
		fmt.Fprintf(&b, "- first_attempt_pass_rate: %s\n", formatRate(m.FirstAttemptPassRate, m.FirstAttemptPasses, m.FirstAttemptTotal))
		fmt.Fprintf(&b, "- recovery_convergence_rate: %s\n", formatRate(m.RecoveryConvergenceRate, m.RecoveredTasks, m.RecoveryTotal))
		fmt.Fprintf(&b, "- review_pass_rate: %s\n", formatRate(m.ReviewPassRate, m.ReviewPasses, m.ReviewTotal))
		fmt.Fprintf(&b, "- challenge_override_rate: %s\n", formatRate(m.ChallengeOverrideRate, m.ChallengeOverrides, m.ReviewChallengeTotal))
		fmt.Fprintf(&b, "- workspace_baseline_coverage: %s\n", formatRate(m.WorkspaceBaselineCoverage, m.WorkspaceBaselineTasks, m.SessionTotal))
		if len(m.BlockedGateDistribution) > 0 {
			fmt.Fprintf(&b, "- blocked_gate_distribution:\n")
			for _, gate := range []string{"build", "review", "review_provider"} {
				if m.BlockedGateDistribution[gate] > 0 {
					fmt.Fprintf(&b, "  - %s: %d\n", gate, m.BlockedGateDistribution[gate])
				}
			}
		}
	}
	return b.String()
}

func formatRate(value float64, numerator int, denominator int) string {
	if denominator == 0 {
		return fmt.Sprintf("n/a (%d/%d)", numerator, denominator)
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", value*100, numerator, denominator)
}
