// Package output formats human-readable command output for the CLI adapter.
package output

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	appbuild "github.com/nilstate/scafld/v2/internal/app/build"
	"github.com/nilstate/scafld/v2/internal/app/envelope"
	appreport "github.com/nilstate/scafld/v2/internal/app/report"
	appreview "github.com/nilstate/scafld/v2/internal/app/review"
	appstatus "github.com/nilstate/scafld/v2/internal/app/status"
	"github.com/nilstate/scafld/v2/internal/core/gate"
	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// EncodeEnvelope writes env as one JSON line, surfacing an encode failure on
// the stream and refusing to report success (exit 0) over corrupt output.
func EncodeEnvelope[T any](w io.Writer, env envelope.Envelope[T], exit int) int {
	if err := json.NewEncoder(w).Encode(env); err != nil {
		fmt.Fprintf(w, "\nerror: encode result envelope: %v\n", err)
		if exit == 0 {
			return 1
		}
	}
	return exit
}

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

// StatusCommandExit maps complete/fail/cancel wrapper errors onto the public
// exit-code table without making the top-level CLI adapter interpret gate data.
func StatusCommandExit(command string, err error, generic int, validation int) int {
	if command == "complete" && GateFailure(err) != nil {
		return validation
	}
	return generic
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

// ConfigGateError wraps config loading failures in the shared repair contract.
func ConfigGateError(err error) error {
	if err == nil {
		err = errors.New("config load failed")
	}
	return gate.New(err, gate.Failure{
		Gate:     "config",
		Status:   "invalid",
		Reason:   "workspace config could not be loaded",
		Evidence: []string{".scafld/config.yaml", ".scafld/config.local.yaml"},
		Expected: "valid scafld config shape",
		Actual:   err.Error(),
		Blockers: []string{"repair workspace config before running lifecycle gates"},
		Next:     "scafld config",
	})
}

// ReviewProviderGateError wraps provider-selection failures in the shared
// review gate repair contract.
func ReviewProviderGateError(err error) error {
	if err == nil {
		err = errors.New("review provider selection failed")
	}
	return gate.New(err, gate.Failure{
		Gate:     "review",
		Status:   "review",
		Reason:   "review provider could not be selected",
		Evidence: []string{".scafld/config.yaml", ".scafld/config.local.yaml", "--provider"},
		Expected: "external review provider configured and available",
		Actual:   err.Error(),
		Blockers: []string{err.Error()},
		Next:     "scafld config",
	})
}

// Gate formats a deterministic gate failure for human CLI output.
func Gate(failure *gate.Failure) string {
	if failure == nil {
		return ""
	}
	var b strings.Builder
	if failure.Gate != "" {
		fmt.Fprintf(&b, "gate: %s\n", failure.Gate)
	}
	if failure.Status != "" {
		fmt.Fprintf(&b, "status: %s\n", failure.Status)
	}
	if failure.Reason != "" {
		fmt.Fprintf(&b, "reason: %s\n", failure.Reason)
	}
	if failure.Expected != "" {
		fmt.Fprintf(&b, "expected: %s\n", failure.Expected)
	}
	if failure.Actual != "" {
		fmt.Fprintf(&b, "actual: %s\n", failure.Actual)
	}
	if len(failure.Evidence) > 0 {
		fmt.Fprintf(&b, "evidence:\n")
		for _, evidence := range failure.Evidence {
			fmt.Fprintf(&b, "- %s\n", evidence)
		}
	}
	if len(failure.Blockers) > 0 {
		fmt.Fprintf(&b, "blockers:\n")
		for _, blocker := range failure.Blockers {
			fmt.Fprintf(&b, "- %s\n", blocker)
		}
	}
	if failure.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", failure.Next)
	}
	return b.String()
}

// Build formats build evidence and repair details when acceptance blocks.
func Build(out appbuild.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "build %s: %d passed, %d failed\n", out.Status, out.Passed, out.Failed)
	if out.Phase != "" {
		fmt.Fprintf(&b, "phase: %s\n", out.Phase)
	}
	if out.Repair != nil {
		b.WriteString(Gate(out.Repair))
	} else if out.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", out.Next)
	}
	return b.String()
}

// Review formats the review gate result so findings are visible in the normal path.
func Review(out appreview.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "review verdict: %s\n", out.Verdict)
	if out.Mode != "" {
		fmt.Fprintf(&b, "review mode: %s\n", out.Mode)
	}
	if out.Provider != "" {
		if out.Model != "" {
			fmt.Fprintf(&b, "review provider: %s:%s\n", out.Provider, out.Model)
		} else {
			fmt.Fprintf(&b, "review provider: %s\n", out.Provider)
		}
	}
	if out.OutputFormat != "" {
		fmt.Fprintf(&b, "review output: %s\n", out.OutputFormat)
	}
	if len(out.Normalizations) > 0 {
		fmt.Fprintf(&b, "review normalizations: %s\n", strings.Join(out.Normalizations, ", "))
	}
	if out.Summary != "" {
		fmt.Fprintf(&b, "summary: %s\n", out.Summary)
	}
	if len(out.Findings) > 0 {
		fmt.Fprintf(&b, "findings:\n")
		for _, finding := range out.Findings {
			blocking := "non-blocking"
			if finding.BlocksCompletion {
				blocking = "blocks completion"
			}
			fmt.Fprintf(&b, "- [%s/%s] %s: %s\n", finding.Severity, blocking, finding.ID, finding.Summary)
			if finding.Location != nil && finding.Location.Path != "" {
				if finding.Location.Line > 0 {
					fmt.Fprintf(&b, "  location: %s:%d\n", finding.Location.Path, finding.Location.Line)
				} else {
					fmt.Fprintf(&b, "  location: %s\n", finding.Location.Path)
				}
			}
			if finding.Evidence != "" {
				fmt.Fprintf(&b, "  evidence: %s\n", finding.Evidence)
			}
			if finding.Validation != "" {
				fmt.Fprintf(&b, "  validation: %s\n", finding.Validation)
			}
		}
	}
	if len(out.AttackLog) > 0 {
		fmt.Fprintf(&b, "attack log:\n")
		for _, attack := range out.AttackLog {
			fmt.Fprintf(&b, "- %s: %s -> %s\n", attack.Target, attack.Attack, attack.Result)
		}
	}
	if out.Repair != nil {
		b.WriteString(Gate(out.Repair))
	} else if out.Next != "" {
		fmt.Fprintf(&b, "next: %s\n", out.Next)
	}
	return b.String()
}

// Status formats status output with the latest review findings when present.
func Status(out appstatus.Output) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %s\nnext: %s\n", out.TaskID, out.Status, out.Next)
	if out.Repair != nil {
		b.WriteString(Gate(out.Repair))
	}
	if out.TaskMaterial != nil {
		fmt.Fprintf(&b, "task material: %s\n", fallback(out.TaskMaterial.MaterialStatus, "projected"))
		if len(out.TaskMaterial.Scope) > 0 {
			fmt.Fprintf(&b, "scope: %s\n", strings.Join(out.TaskMaterial.Scope, ", "))
		}
		if len(out.TaskMaterial.TaskChanges) > 0 {
			fmt.Fprintf(&b, "task changes:\n")
			for _, change := range out.TaskMaterial.TaskChanges {
				fmt.Fprintf(&b, "- %s\n", change)
			}
		}
		if len(out.TaskMaterial.AmbientDrift) > 0 {
			fmt.Fprintf(&b, "ambient drift:\n")
			for _, change := range out.TaskMaterial.AmbientDrift {
				fmt.Fprintf(&b, "- %s\n", change)
			}
		}
	}
	if out.Completion != nil {
		fmt.Fprintf(&b, "completion authority: %s", out.Completion.Status)
		if out.Completion.Kind != "" {
			fmt.Fprintf(&b, " (%s)", out.Completion.Kind)
		}
		b.WriteString("\n")
		if out.Completion.Provider != "" || out.Completion.Verdict != "" {
			fmt.Fprintf(&b, "authority review: %s", out.Completion.Verdict)
			if out.Completion.Provider != "" {
				fmt.Fprintf(&b, " by %s", out.Completion.Provider)
			}
			b.WriteString("\n")
		}
		if out.Completion.Summary != "" {
			fmt.Fprintf(&b, "authority summary: %s\n", out.Completion.Summary)
		}
		if out.Completion.Status == "invalid" && out.Completion.Reason != "" {
			fmt.Fprintf(&b, "authority error: %s\n", out.Completion.Reason)
			if out.Completion.Actual != "" {
				fmt.Fprintf(&b, "authority actual: %s\n", out.Completion.Actual)
			}
		}
	}
	if out.Review.Running {
		fmt.Fprintf(&b, "review: running\n")
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	} else if out.Review.Verdict != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.Verdict)
		if out.Review.Mode != "" {
			fmt.Fprintf(&b, "review mode: %s\n", out.Review.Mode)
		}
		if out.Review.Summary != "" {
			fmt.Fprintf(&b, "summary: %s\n", out.Review.Summary)
		}
		for _, finding := range out.Review.Findings {
			blocking := "non-blocking"
			if finding.BlocksCompletion {
				blocking = "blocks completion"
			}
			fmt.Fprintf(&b, "- [%s/%s] %s: %s\n", finding.Severity, blocking, finding.ID, finding.Summary)
			if finding.Validation != "" {
				fmt.Fprintf(&b, "  validation: %s\n", finding.Validation)
			}
		}
	} else if out.Review.AttemptStatus != "" {
		fmt.Fprintf(&b, "review: %s\n", out.Review.AttemptStatus)
		if out.Review.Reason != "" {
			fmt.Fprintf(&b, "reason: %s\n", out.Review.Reason)
		}
	}
	return b.String()
}

func fallback(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
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
	fmt.Fprintf(&b, "metrics:\n")
	fmt.Fprintf(&b, "- first_attempt_pass_rate: %s\n", formatRate(m.FirstAttemptPassRate, m.FirstAttemptPasses, m.FirstAttemptTotal))
	fmt.Fprintf(&b, "- recovery_convergence_rate: %s\n", formatRate(m.RecoveryConvergenceRate, m.RecoveredTasks, m.RecoveryTotal))
	fmt.Fprintf(&b, "- review_pass_rate: %s\n", formatRate(m.ReviewPassRate, m.ReviewPasses, m.ReviewTotal))
	fmt.Fprintf(&b, "- review_dossier_coverage: %s\n", formatRate(m.ReviewDossierCoverage, m.ReviewDossierTotal, m.ReviewTotal))
	if m.ReviewDossierTotal > 0 {
		fmt.Fprintf(&b, "- review_findings_total: %d\n", m.ReviewFindingsTotal)
		fmt.Fprintf(&b, "- review_open_blockers_total: %d\n", m.ReviewOpenBlockersTotal)
		fmt.Fprintf(&b, "- review_attack_angles_total: %d\n", m.ReviewAttackAnglesTotal)
		if len(m.ReviewModeDistribution) > 0 {
			fmt.Fprintf(&b, "- review_mode_distribution:\n")
			for _, mode := range []string{"discover", "verify"} {
				if m.ReviewModeDistribution[mode] > 0 {
					fmt.Fprintf(&b, "  - %s: %d\n", mode, m.ReviewModeDistribution[mode])
				}
			}
		}
	}
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
	return b.String()
}

func formatRate(value float64, numerator int, denominator int) string {
	if denominator == 0 {
		return fmt.Sprintf("n/a (%d/%d)", numerator, denominator)
	}
	return fmt.Sprintf("%.1f%% (%d/%d)", value*100, numerator, denominator)
}
