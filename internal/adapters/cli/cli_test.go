package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunHelpAndVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "root help", args: nil, want: "Commands:"},
		{name: "flag help", args: []string{"--help"}, want: "Usage:"},
		{name: "version", args: []string{"--version"}, want: displayVersion()},
		{name: "command help", args: []string{"plan", "--help"}, want: "scafld plan"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := Run(context.Background(), tc.args, &stdout, &stderr)
			if code != ExitSuccess {
				t.Fatalf("Run() exit = %d, want %d; stderr=%q", code, ExitSuccess, stderr.String())
			}
			if !strings.Contains(stdout.String(), tc.want) {
				t.Fatalf("stdout %q does not contain %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestDisplayVersionUsesInjectedVersion(t *testing.T) {
	old := version
	t.Cleanup(func() { version = old })
	version = "vtest-release"
	if got := displayVersion(); got != "test-release" {
		t.Fatalf("displayVersion() = %q, want test-release", got)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"missing"}, &stdout, &stderr)
	if code != ExitInvalid {
		t.Fatalf("Run() exit = %d, want %d", code, ExitInvalid)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr %q does not explain the failure", stderr.String())
	}
}

func TestRunInit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"init", "--root", root}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(init) exit = %d, want %d; stderr=%q", code, ExitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "initialized scafld workspace") {
		t.Fatalf("stdout %q does not confirm init", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".scafld", "config.yaml")); err != nil {
		t.Fatalf("config not created: %v", err)
	}
	for _, rel := range []string{
		".scafld/core/config.yaml",
		".scafld/core/prompts/harden.md",
		".scafld/core/schemas/spec.json",
		".scafld/core/scripts/scafld-codex-build.sh",
		".scafld/prompts/harden.md",
		".scafld/config.local.yaml",
		"AGENTS.md",
		"CLAUDE.md",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s missing: %v", rel, err)
		}
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(agents), "scafld:contract") || !strings.Contains(string(agents), "scafld Agent Contract") || !strings.Contains(string(agents), "Do Not") {
		t.Fatalf("AGENTS.md does not include the scafld contract:\n%s", agents)
	}
	if info, err := os.Stat(filepath.Join(root, ".scafld", "core", "scripts", "scafld-codex-build.sh")); err != nil || info.Mode()&0o111 == 0 {
		t.Fatalf("core script should be executable, info=%v err=%v", info, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".ai")); !os.IsNotExist(err) {
		t.Fatalf(".ai should not be created, stat error = %v", err)
	}
}

func TestRunInitCanSkipAgentDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md should not be created with --no-agent-docs, stat error = %v", err)
	}
}

func TestRunUpdateRefreshesCoreButPreservesProjectPrompts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	corePrompt := filepath.Join(root, ".scafld", "core", "prompts", "harden.md")
	projectPrompt := filepath.Join(root, ".scafld", "prompts", "harden.md")
	if err := os.WriteFile(corePrompt, []byte("stale core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPrompt, []byte("custom project prompt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout := runCLI(t, []string{"update", "--root", root})
	if !strings.Contains(stdout, "refreshed scafld core") {
		t.Fatalf("update stdout %q does not confirm refresh", stdout)
	}
	coreData, err := os.ReadFile(corePrompt)
	if err != nil {
		t.Fatal(err)
	}
	projectData, err := os.ReadFile(projectPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(coreData), "HARDEN MODE TEMPLATE") {
		t.Fatalf("core prompt was not refreshed:\n%s", coreData)
	}
	if string(projectData) != "custom project prompt\n" {
		t.Fatalf("project prompt was overwritten:\n%s", projectData)
	}
}

func TestRunUpdateRefreshesManagedAgentDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	agentsPath := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agentsPath, []byte("# scafld Agent Contract\n\nstale\n\n# Project Rules\n\nproject note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCLI(t, []string{"update", "--root", root})
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "stale") || !strings.Contains(text, "scafld Agent Contract") || !strings.Contains(text, "project note") {
		t.Fatalf("managed AGENTS.md block was not refreshed cleanly:\n%s", text)
	}
}

func TestRunHardenLifecycle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	runCLI(t, []string{"plan", "--root", root, "harden-task", "--command", "go version"})

	stdout := runCLI(t, []string{"harden", "--root", root, "harden-task"})
	if !strings.Contains(stdout, "# HARDEN MODE TEMPLATE") || !strings.Contains(stdout, "when done, mark the round passed: scafld harden harden-task --mark-passed") {
		t.Fatalf("harden stdout %q does not enter harden mode", stdout)
	}
	specPath := filepath.Join(root, ".scafld", "specs", "drafts", "harden-task.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "harden_status: in_progress") || !strings.Contains(text, "### round-1") || !strings.Contains(text, "Status: in_progress") {
		t.Fatalf("spec was not opened for hardening:\n%s", text)
	}

	stdout = runCLI(t, []string{"harden", "--root", root, "harden-task", "--mark-passed"})
	if !strings.Contains(stdout, "harden passed: harden-task") {
		t.Fatalf("mark-passed stdout %q does not confirm pass", stdout)
	}
	data, err = os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	text = string(data)
	if !strings.Contains(text, "harden_status: passed") || !strings.Contains(text, "### round-1") || !strings.Contains(text, "Status: passed") || !strings.Contains(text, "Ended: ") {
		t.Fatalf("spec was not marked hardened:\n%s", text)
	}
}

func TestExitCodeTable(t *testing.T) {
	t.Parallel()

	want := map[string]int{
		"success":    0,
		"generic":    1,
		"invalid":    2,
		"validation": 3,
		"review":     4,
		"cancelled":  5,
		"workspace":  6,
	}
	got := map[string]int{
		"success":    ExitSuccess,
		"generic":    ExitGeneric,
		"invalid":    ExitInvalid,
		"validation": ExitValidation,
		"review":     ExitReview,
		"cancelled":  ExitCancelled,
		"workspace":  ExitWorkspace,
	}
	for name, wantCode := range want {
		if got[name] != wantCode {
			t.Fatalf("%s exit code = %d, want %d", name, got[name], wantCode)
		}
	}
}

func runCLI(t *testing.T, args []string) string {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), args, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run(%v) exit = %d, want %d; stderr=%q", args, code, ExitSuccess, stderr.String())
	}
	return stdout.String()
}

func TestCancelledErrorsUseCancelledExitCode(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := failOut(&stderr, context.Canceled, ExitGeneric, false)
	if code != ExitCancelled {
		t.Fatalf("exit = %d, want %d", code, ExitCancelled)
	}
}
