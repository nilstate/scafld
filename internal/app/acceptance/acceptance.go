package acceptance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	coreacceptance "github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/execution"
)

// Runner executes acceptance commands.
type Runner interface {
	Run(context.Context, execution.Request) (execution.Result, error)
}

// EvaluateInput describes criteria and execution settings for one acceptance pass.
type EvaluateInput struct {
	Criteria      []Criterion
	WorkDir       string
	Env           []string
	Timeout       time.Duration
	IdleTimeout   time.Duration
	DiagnosticDir string
	// FailFast stops evaluation after the first non-pass criterion. The gate sets
	// it so a red criterion cannot run later commands (which could mutate the
	// workspace); build leaves it false to record every criterion.
	FailFast bool
}

// Criterion is the app-level acceptance criterion shape shared by build and gate.
type Criterion struct {
	ID           string
	Type         string
	Command      string
	ExpectedKind string
}

// CriterionResult is immutable evidence for one evaluated criterion.
type CriterionResult struct {
	ID             string
	Type           string
	Command        string
	ExpectedKind   string
	Status         string
	ExitCode       int
	Reason         string
	DiagnosticPath string
	Evidence       string
	StdoutDigest   string
	StderrDigest   string
	StartedAt      time.Time
	EndedAt        time.Time
}

// EvaluateOutput summarizes evaluated criteria.
type EvaluateOutput struct {
	Results []CriterionResult
	Passed  bool
}

// Evaluate runs and evaluates criteria without mutating lifecycle or session state.
func Evaluate(ctx context.Context, runner Runner, input EvaluateInput) EvaluateOutput {
	results := make([]CriterionResult, 0, len(input.Criteria))
	passed := true
	for _, criterion := range input.Criteria {
		result := evaluateCriterion(ctx, runner, criterion, input)
		results = append(results, result)
		if result.Status != "pass" {
			passed = false
			if input.FailFast {
				break
			}
		}
	}
	return EvaluateOutput{Results: results, Passed: passed}
}

func evaluateCriterion(ctx context.Context, runner Runner, criterion Criterion, input EvaluateInput) CriterionResult {
	started := time.Now().UTC()
	if strings.TrimSpace(criterion.Command) == "" {
		evaluation := coreacceptance.Evaluate(coreacceptance.ExpectedKind(criterion.ExpectedKind), coreacceptance.Evidence{})
		ended := time.Now().UTC()
		return CriterionResult{
			ID:           criterion.ID,
			Type:         criterion.Type,
			Command:      criterion.Command,
			ExpectedKind: criterion.ExpectedKind,
			Status:       evaluation.Status,
			Reason:       evaluation.Reason,
			StdoutDigest: digest(""),
			StderrDigest: digest(""),
			StartedAt:    started,
			EndedAt:      ended,
		}
	}
	result, runErr := run(ctx, runner, criterion.Command, input)
	ended := time.Now().UTC()
	evidenceOutput := result.Output
	if isBrowserCriterion(criterion) {
		evidenceOutput = result.Stdout
	}
	evaluation := coreacceptance.Evaluate(coreacceptance.ExpectedKind(criterion.ExpectedKind), coreacceptance.Evidence{
		ExitCode:   result.ExitCode,
		Output:     evidenceOutput,
		Command:    criterion.Command,
		Diagnostic: result.Output,
	})
	if runErr != nil {
		evaluation.Status = "fail"
		evaluation.Reason = runErr.Error()
	}
	entryOutput := result.Output
	if isBrowserCriterion(criterion) && strings.TrimSpace(result.Stdout) != "" {
		entryOutput = result.Stdout
	}
	return CriterionResult{
		ID:             criterion.ID,
		Type:           criterion.Type,
		Command:        criterion.Command,
		ExpectedKind:   criterion.ExpectedKind,
		Status:         evaluation.Status,
		ExitCode:       result.ExitCode,
		Reason:         evaluation.Reason,
		DiagnosticPath: result.DiagnosticPath,
		Evidence:       snippet(entryOutput),
		StdoutDigest:   digest(result.Stdout),
		StderrDigest:   digest(result.Stderr),
		StartedAt:      started,
		EndedAt:        ended,
	}
}

func run(ctx context.Context, runner Runner, command string, input EvaluateInput) (execution.Result, error) {
	if runner == nil {
		return execution.Result{}, errors.New("missing acceptance runner")
	}
	return runner.Run(ctx, execution.Request{
		Command:     command,
		CWD:         input.WorkDir,
		Env:         input.Env,
		Timeout:     input.Timeout,
		IdleTimeout: input.IdleTimeout,
	})
}

func isBrowserCriterion(criterion Criterion) bool {
	return criterion.Type == "browser" || criterion.ExpectedKind == string(coreacceptance.ExpectedBrowserEvidence)
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func snippet(s string) string {
	if len(s) > 1000 {
		return s[:1000]
	}
	return s
}
