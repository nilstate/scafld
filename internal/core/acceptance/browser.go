package acceptance

import (
	"encoding/json"
	"fmt"
	"strings"
)

type browserEvidence struct {
	URL           string            `json:"url"`
	Viewport      string            `json:"viewport"`
	Auth          browserAuth       `json:"auth"`
	Screenshots   []browserArtifact `json:"screenshots"`
	Traces        []browserArtifact `json:"traces"`
	Videos        []browserArtifact `json:"videos"`
	Artifacts     []browserArtifact `json:"artifacts"`
	ConsoleErrors []string          `json:"console_errors"`
	NetworkErrors []string          `json:"network_errors"`
}

type browserAuth struct {
	Mode     string `json:"mode"`
	Artifact string `json:"artifact"`
}

type browserArtifact struct {
	Path        string `json:"path"`
	Description string `json:"description"`
	SHA256      string `json:"sha256"`
}

func evaluateBrowserEvidence(evidence Evidence) Result {
	if evidence.ExitCode != 0 {
		reason := fmt.Sprintf("browser command exited with code %d", evidence.ExitCode)
		if hint, ok := playwrightInstallHint(evidence.Command, evidence.Diagnostic); ok {
			reason += "; " + hint
		}
		return Result{Status: "fail", Reason: reason}
	}
	output := strings.TrimSpace(evidence.Output)
	if output == "" {
		reason := "browser evidence JSON was empty"
		if hint, ok := playwrightInstallHint(evidence.Command, evidence.Diagnostic); ok {
			reason += "; " + hint
		}
		return Result{Status: "fail", Reason: reason}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		return Result{Status: "fail", Reason: "browser evidence JSON invalid: " + err.Error()}
	}
	for _, key := range []string{"url", "viewport", "console_errors", "network_errors"} {
		if _, ok := raw[key]; !ok {
			return Result{Status: "fail", Reason: "browser evidence missing " + key}
		}
	}
	var packet browserEvidence
	if err := json.Unmarshal([]byte(output), &packet); err != nil {
		return Result{Status: "fail", Reason: "browser evidence JSON invalid: " + err.Error()}
	}
	if strings.TrimSpace(packet.URL) == "" {
		return Result{Status: "fail", Reason: "browser evidence missing url"}
	}
	if strings.TrimSpace(packet.Viewport) == "" {
		return Result{Status: "fail", Reason: "browser evidence missing viewport"}
	}
	if _, ok := raw["auth"]; ok && strings.TrimSpace(packet.Auth.Mode) == "" {
		return Result{Status: "fail", Reason: "browser evidence auth.mode is required when auth is present"}
	}
	if len(packet.ConsoleErrors) > 0 {
		return Result{Status: "fail", Reason: "browser evidence recorded console errors: " + strings.Join(packet.ConsoleErrors, "; ")}
	}
	if len(packet.NetworkErrors) > 0 {
		return Result{Status: "fail", Reason: "browser evidence recorded network errors: " + strings.Join(packet.NetworkErrors, "; ")}
	}
	if artifactProblem := validateBrowserArtifacts(packet); artifactProblem != "" {
		return Result{Status: "fail", Reason: artifactProblem}
	}
	return Result{Status: "pass", Reason: "browser evidence accepted for " + packet.URL + " at " + packet.Viewport}
}

func validateBrowserArtifacts(packet browserEvidence) string {
	groups := [][]browserArtifact{packet.Screenshots, packet.Traces, packet.Videos, packet.Artifacts}
	var sawArtifact bool
	for _, group := range groups {
		for _, artifact := range group {
			sawArtifact = true
			if strings.TrimSpace(artifact.Path) == "" {
				return "browser evidence artifact path is required"
			}
		}
	}
	if !sawArtifact {
		return "browser evidence needs at least one screenshot, trace, video, or artifact"
	}
	return ""
}

func playwrightInstallHint(command string, diagnostic string) (string, bool) {
	haystack := strings.ToLower(command + "\n" + diagnostic)
	if !strings.Contains(haystack, "playwright") {
		return "", false
	}
	indicators := []string{
		"command not found",
		"not recognized as an internal or external command",
		"cannot find module '@playwright/test'",
		"cannot find package '@playwright/test'",
		"executable doesn't exist",
		"browser executable not found",
		"please run the following command",
		"playwright install",
		"no executable found",
	}
	for _, indicator := range indicators {
		if strings.Contains(haystack, indicator) {
			return "Playwright appears unavailable for this browser criterion; install project dependencies and browser binaries, then rerun the same command (for many Node projects: install dependencies, then run `npx playwright install --with-deps` or the equivalent `pnpm/yarn/bun` exec command)", true
		}
	}
	return "", false
}
