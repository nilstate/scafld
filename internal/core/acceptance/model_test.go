package acceptance

import (
	"strings"
	"testing"
)

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

func TestBrowserEvidencePassesWithArtifact(t *testing.T) {
	t.Parallel()

	output := `{"url":"http://localhost:3000/dashboard","viewport":"1440x900","auth":{"mode":"storage_state","artifact":".auth/user.json"},"screenshots":[{"path":".scafld/runs/task/dashboard.png","description":"dashboard loaded"}],"console_errors":[],"network_errors":[]}`
	got := Evaluate(ExpectedBrowserEvidence, Evidence{ExitCode: 0, Output: output})
	if got.Status != "pass" || got.Reason != "browser evidence accepted for http://localhost:3000/dashboard at 1440x900" {
		t.Fatalf("browser evidence result = %+v", got)
	}
}

func TestBrowserEvidenceRejectsMissingPacket(t *testing.T) {
	t.Parallel()

	got := Evaluate(ExpectedBrowserEvidence, Evidence{ExitCode: 0, Output: ""})
	if got.Status != "fail" || got.Reason != "browser evidence JSON was empty" {
		t.Fatalf("empty browser evidence result = %+v", got)
	}
}

func TestBrowserEvidenceRejectsConsoleAndNetworkErrors(t *testing.T) {
	t.Parallel()

	output := `{"url":"http://localhost:3000/dashboard","viewport":"1440x900","screenshots":[{"path":"dashboard.png"}],"console_errors":["TypeError: broken"],"network_errors":[]}`
	got := Evaluate(ExpectedBrowserEvidence, Evidence{ExitCode: 0, Output: output})
	if got.Status != "fail" || got.Reason != "browser evidence recorded console errors: TypeError: broken" {
		t.Fatalf("console error result = %+v", got)
	}

	output = `{"url":"http://localhost:3000/dashboard","viewport":"1440x900","screenshots":[{"path":"dashboard.png"}],"console_errors":[],"network_errors":["GET /api 500"]}`
	got = Evaluate(ExpectedBrowserEvidence, Evidence{ExitCode: 0, Output: output})
	if got.Status != "fail" || got.Reason != "browser evidence recorded network errors: GET /api 500" {
		t.Fatalf("network error result = %+v", got)
	}
}

func TestBrowserEvidenceRejectsMissingArtifacts(t *testing.T) {
	t.Parallel()

	output := `{"url":"http://localhost:3000/dashboard","viewport":"1440x900","console_errors":[],"network_errors":[]}`
	got := Evaluate(ExpectedBrowserEvidence, Evidence{ExitCode: 0, Output: output})
	if got.Status != "fail" || got.Reason != "browser evidence needs at least one screenshot, trace, video, or artifact" {
		t.Fatalf("missing artifact result = %+v", got)
	}
}

func TestBrowserEvidenceAddsPlaywrightInstallHint(t *testing.T) {
	t.Parallel()

	got := Evaluate(ExpectedBrowserEvidence, Evidence{
		ExitCode:   1,
		Command:    "npx playwright test",
		Diagnostic: "Error: browserType.launch: Executable doesn't exist at /ms-playwright/chromium",
	})
	if got.Status != "fail" {
		t.Fatalf("playwright failure status = %+v", got)
	}
	if want := "Playwright appears unavailable"; !strings.Contains(got.Reason, want) {
		t.Fatalf("reason %q missing %q", got.Reason, want)
	}
}
