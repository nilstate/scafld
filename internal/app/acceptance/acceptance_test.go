package acceptance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	coreacceptance "github.com/nilstate/scafld/v2/internal/core/acceptance"
	"github.com/nilstate/scafld/v2/internal/core/execution"
)

type fakeRunner struct {
	result   execution.Result
	err      error
	requests []execution.Request
}

func (f *fakeRunner) Run(_ context.Context, req execution.Request) (execution.Result, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func TestEvaluateRunsCommandCriteria(t *testing.T) {
	t.Parallel()

	output := strings.Repeat("x", 1005)
	runner := &fakeRunner{result: execution.Result{
		ExitCode:       0,
		Stdout:         "stdout",
		Stderr:         "stderr",
		Output:         output,
		DiagnosticPath: "/tmp/scafld-diagnostic.txt",
	}}
	out := Evaluate(context.Background(), runner, EvaluateInput{
		Criteria: []Criterion{{
			ID:           "ac1",
			Command:      "go test ./...",
			ExpectedKind: string(coreacceptance.ExpectedExitCodeZero),
		}},
		WorkDir:     "/repo",
		Env:         []string{"PATH=/bin"},
		Timeout:     2 * time.Minute,
		IdleTimeout: 30 * time.Second,
	})

	if !out.Passed || len(out.Results) != 1 {
		t.Fatalf("output = %+v, want one passing result", out)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("requests = %+v, want one runner call", runner.requests)
	}
	req := runner.requests[0]
	if req.Command != "go test ./..." || req.CWD != "/repo" || len(req.Env) != 1 || req.Env[0] != "PATH=/bin" || req.Timeout != 2*time.Minute || req.IdleTimeout != 30*time.Second {
		t.Fatalf("request = %+v, want configured command request", req)
	}
	result := out.Results[0]
	if result.Status != "pass" || result.Reason != "exit code was 0" || result.ExitCode != 0 || result.DiagnosticPath != "/tmp/scafld-diagnostic.txt" {
		t.Fatalf("result = %+v, want successful criterion evidence", result)
	}
	if result.Evidence != output[:1000] {
		t.Fatalf("evidence length = %d, want truncated output snippet", len(result.Evidence))
	}
	if result.StdoutDigest != digestForTest("stdout") || result.StderrDigest != digestForTest("stderr") {
		t.Fatalf("digests = %s/%s, want stdout/stderr sha256", result.StdoutDigest, result.StderrDigest)
	}
	if result.StartedAt.IsZero() || result.EndedAt.IsZero() || result.EndedAt.Before(result.StartedAt) {
		t.Fatalf("timestamps = %s -> %s, want ordered non-zero times", result.StartedAt, result.EndedAt)
	}
}

func TestEvaluateBrowserCriteriaUsesStdoutEvidence(t *testing.T) {
	t.Parallel()

	evidence := `{"url":"http://localhost:3000/dashboard","viewport":"1440x900","screenshots":[{"path":".scafld/runs/task/dashboard.png"}],"console_errors":[],"network_errors":[]}`
	runner := &fakeRunner{result: execution.Result{
		ExitCode: 0,
		Stdout:   evidence,
		Stderr:   "dev server log line",
		Output:   "dev server log line\n" + evidence,
	}}
	out := Evaluate(context.Background(), runner, EvaluateInput{Criteria: []Criterion{{
		ID:           "browser",
		Type:         "browser",
		Command:      "npm run browser:check",
		ExpectedKind: string(coreacceptance.ExpectedBrowserEvidence),
	}}})

	if !out.Passed || len(out.Results) != 1 {
		t.Fatalf("output = %+v, want browser pass", out)
	}
	result := out.Results[0]
	if result.Status != "pass" || result.Reason != "browser evidence accepted for http://localhost:3000/dashboard at 1440x900" {
		t.Fatalf("result = %+v, want accepted browser evidence", result)
	}
	if result.Evidence != evidence {
		t.Fatalf("evidence = %q, want stdout browser packet", result.Evidence)
	}
}

func TestEvaluateRunErrorOverridesMatcherReason(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		result: execution.Result{ExitCode: 0, Output: "ok"},
		err:    errors.New("process timeout"),
	}
	out := Evaluate(context.Background(), runner, EvaluateInput{Criteria: []Criterion{{
		ID:           "ac1",
		Command:      "go test ./...",
		ExpectedKind: string(coreacceptance.ExpectedExitCodeZero),
	}}})

	if out.Passed {
		t.Fatalf("output = %+v, want failed pass flag", out)
	}
	result := out.Results[0]
	if result.Status != "fail" || result.Reason != "process timeout" {
		t.Fatalf("result = %+v, want runner error reason", result)
	}
}

func TestEvaluateEmptyCommandCriteriaFailClosedExceptManualEvidence(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	out := Evaluate(context.Background(), runner, EvaluateInput{Criteria: []Criterion{
		{
			ID:           "empty-exit-zero",
			ExpectedKind: string(coreacceptance.ExpectedExitCodeZero),
		},
		{
			ID:           "manual",
			Type:         "manual",
			ExpectedKind: string(coreacceptance.ExpectedManual),
		},
	}})

	if len(runner.requests) != 0 {
		t.Fatalf("requests = %+v, want empty-command criteria not to run", runner.requests)
	}
	if out.Passed {
		t.Fatalf("output = %+v, want manual criterion to block pass flag", out)
	}
	if out.Results[0].Status != "fail" || out.Results[0].Reason != "criterion command is empty" {
		t.Fatalf("empty command result = %+v, want fail-closed empty command", out.Results[0])
	}
	if out.Results[1].Status != "pending" || out.Results[1].Reason != "manual criterion requires human evidence" {
		t.Fatalf("manual result = %+v, want manual pending semantics", out.Results[1])
	}
}

func TestEvaluateBrowserCriteriaReportsPlaywrightInstallHelp(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{
		ExitCode: 1,
		Stderr:   "Error: browserType.launch: Executable doesn't exist at /ms-playwright/chromium\nPlease run the following command: npx playwright install",
		Output:   "Error: browserType.launch: Executable doesn't exist at /ms-playwright/chromium\nPlease run the following command: npx playwright install",
	}}
	out := Evaluate(context.Background(), runner, EvaluateInput{Criteria: []Criterion{{
		ID:           "browser",
		Type:         "browser",
		Command:      "npx playwright test",
		ExpectedKind: string(coreacceptance.ExpectedBrowserEvidence),
	}}})

	if out.Passed {
		t.Fatalf("output = %+v, want browser failure", out)
	}
	if !strings.Contains(out.Results[0].Reason, "Playwright appears unavailable") {
		t.Fatalf("reason = %q, want Playwright install help", out.Results[0].Reason)
	}
}

func digestForTest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestEvaluateFailFastStopsAfterFirstFailure(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{result: execution.Result{ExitCode: 1}}
	out := Evaluate(context.Background(), runner, EvaluateInput{
		Criteria: []Criterion{
			{ID: "a", Command: "first", ExpectedKind: "exit_code_zero"},
			{ID: "b", Command: "second", ExpectedKind: "exit_code_zero"},
		},
		FailFast: true,
	})
	if out.Passed {
		t.Fatal("expected acceptance to fail")
	}
	if len(runner.requests) != 1 || len(out.Results) != 1 {
		t.Fatalf("fail-fast must stop after the first failure: requests=%d results=%d", len(runner.requests), len(out.Results))
	}
}
