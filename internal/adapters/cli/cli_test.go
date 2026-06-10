package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nilstate/scafld/v2/internal/adapters/trustcheck"
	"github.com/nilstate/scafld/v2/internal/core/gate"
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
		{name: "harden help", args: []string{"harden", "--help"}, want: "Required observation dimensions"},
		{name: "review help", args: []string{"review", "--help"}, want: "--review-scope"},
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

func TestFailOutJSONIncludesGateFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := failOut(&stderr, gate.New(errors.New("review gate has not passed"), gate.Failure{
		Gate:     "complete",
		Status:   "review",
		Reason:   "latest review gate has not passed",
		Expected: "review verdict pass",
		Actual:   "review verdict fail",
		Blockers: []string{"blocking finding"},
		Next:     "scafld review task",
	}), ExitValidation, true)
	if code != ExitValidation {
		t.Fatalf("exit = %d", code)
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Gate gate.Failure `json:"gate"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK || payload.Error.Gate.Gate != "complete" || payload.Error.Gate.Next != "scafld review task" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestFailOutHumanIncludesGateFailure(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := failOut(&stderr, gate.New(errors.New("review gate has not passed"), gate.Failure{
		Gate:     "complete",
		Status:   "review",
		Reason:   "latest review gate has not passed",
		Evidence: []string{"session review entries"},
		Expected: "review verdict pass",
		Actual:   "review verdict fail",
		Blockers: []string{"blocking finding"},
		Next:     "scafld review task",
	}), ExitValidation, false)
	if code != ExitValidation {
		t.Fatalf("exit = %d", code)
	}
	for _, want := range []string{
		"error: review gate has not passed",
		"gate: complete",
		"expected: review verdict pass",
		"actual: review verdict fail",
		"evidence:",
		"- session review entries",
		"blockers:",
		"- blocking finding",
		"next: scafld review task",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestBlockedBuildHumanOutputUsesGateFailureContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "blocked-build", "--command", "false"})
	runCLI(t, []string{"approve", "--root", root, "blocked-build"})
	runCLI(t, []string{"build", "--root", root, "blocked-build"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"build", "--root", root, "blocked-build"}, &stdout, &stderr)
	if code != ExitValidation {
		t.Fatalf("Run(build) exit = %d, want %d; stdout=%q stderr=%q", code, ExitValidation, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"build blocked:",
		"gate: build",
		"status: blocked",
		"reason:",
		"evidence:",
		"expected: all acceptance criteria pass",
		"actual:",
		"blockers:",
		"next: scafld handoff blocked-build",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestListDoesNotReadSessionLedgers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "metadata-only-list", "--title", "Metadata Only List", "--command", "true"})
	sessionPath := filepath.Join(root, ".scafld", "runs", "metadata-only-list", "session.json")
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runCLI(t, []string{"list", "--root", root})
	if !strings.Contains(out, "metadata-only-list") {
		t.Fatalf("list output missing task:\n%s", out)
	}
}

func TestStoresWireSessionTrustChecker(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	defaultKeysPath := filepath.Join(root, ".scafld", "trusted-keys.json")
	customKeysPath := filepath.Join(root, ".scafld", "custom-trusted-keys.json")
	keys, err := os.ReadFile(defaultKeysPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultKeysPath, []byte("{bad json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(customKeysPath, keys, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte("verify:\n  trusted_keys_path: .scafld/custom-trusted-keys.json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, sessions, code, err := stores(context.Background(), options{Root: root})
	if err != nil {
		t.Fatalf("stores() error = %v", err)
	}
	if code != ExitSuccess {
		t.Fatalf("stores() code = %d, want %d", code, ExitSuccess)
	}
	if sessions.TrustChecker == nil {
		t.Fatal("stores() returned a session store without a receipt trust checker")
	}
	checker, ok := sessions.TrustChecker.(trustcheck.Checker)
	if !ok {
		t.Fatalf("stores() trust checker type = %T, want trustcheck.Checker", sessions.TrustChecker)
	}
	if checker.LoadErr != nil {
		t.Fatalf("stores() did not use configured trusted keys path: %v", checker.LoadErr)
	}
}

func TestBuildConfigFailureUsesGateFailureContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte("invariants:\n  canonical:\n    - bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"build", "--root", root, "task"}, &stdout, &stderr)
	if code != ExitGeneric {
		t.Fatalf("Run(build) exit = %d, want %d; stdout=%q stderr=%q", code, ExitGeneric, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"gate: config",
		"status: invalid",
		"reason: workspace config could not be loaded",
		"evidence:",
		".scafld/config.yaml",
		"expected: valid scafld config shape",
		"actual:",
		"blockers:",
		"next: scafld config",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestReviewProviderSelectionFailureUsesGateFailureContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"review", "--root", root, "--provider", "command", "task"}, &stdout, &stderr)
	if code != ExitInvalid {
		t.Fatalf("Run(review) exit = %d, want %d; stdout=%q stderr=%q", code, ExitInvalid, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"gate: review",
		"status: review",
		"reason: review provider could not be selected",
		"evidence:",
		".scafld/config.yaml",
		"expected: external review provider configured and available",
		"actual: --provider=command requires --provider-command",
		"blockers:",
		"next: scafld config",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestReviewProviderSelectionFailureJSONIncludesGateFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"review", "--root", root, "--provider", "command", "--json", "task"}, &stdout, &stderr)
	if code != ExitInvalid {
		t.Fatalf("Run(review) exit = %d, want %d; stdout=%q stderr=%q", code, ExitInvalid, stdout.String(), stderr.String())
	}
	var payload struct {
		OK    bool `json:"ok"`
		Error struct {
			Gate gate.Failure `json:"gate"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.OK || payload.Error.Gate.Gate != "review" || payload.Error.Gate.Actual != "--provider=command requires --provider-command" {
		t.Fatalf("payload = %+v", payload)
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
	config, err := os.ReadFile(filepath.Join(root, ".scafld", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "Keep this file sparse") || strings.Contains(string(config), "adversarial_passes:") {
		t.Fatalf("project config should be sparse:\n%s", config)
	}
	for _, rel := range []string{
		".gitignore",
		".scafld/core/config.yaml",
		".scafld/core/prompts/harden.md",
		".scafld/core/schemas/spec.json",
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
	if info, err := os.Lstat(filepath.Join(root, "AGENTS.md")); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("AGENTS.md should be a real root copy, info=%v err=%v", info, err)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gitignore), "# scafld runtime state") || !strings.Contains(string(gitignore), ".scafld/config.local.yaml") {
		t.Fatalf(".gitignore does not include scafld rules:\n%s", gitignore)
	}
	if _, err := os.Stat(filepath.Join(root, ".scafld", "core", "scripts", "scafld-codex-build.sh")); !os.IsNotExist(err) {
		t.Fatalf("default init should not install optional lifecycle helper script, err=%v", err)
	}
}

func TestRunConfigWritesProposal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	if err := os.WriteFile(filepath.Join(root, "Makefile"), []byte("check:\n\tgo test ./...\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	archPath := filepath.Join(root, "internal", "arch", "architecture_test.go")
	if err := os.MkdirAll(filepath.Dir(archPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archPath, []byte("package arch\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := runCLI(t, []string{"config", "--root", root})
	if !strings.Contains(out, "CONFIG MODE") || !strings.Contains(out, ".scafld/config.proposed.yaml") {
		t.Fatalf("config output = %q", out)
	}
	proposal, err := os.ReadFile(filepath.Join(root, ".scafld", "config.proposed.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(proposal), "agent_instructions:") ||
		!strings.Contains(string(proposal), "full_check") ||
		!strings.Contains(string(proposal), "architecture_boundaries") {
		t.Fatalf("proposal missing grounded suggestions:\n%s", proposal)
	}
}

func TestRunInitIsIdempotent(t *testing.T) {
	t.Setenv("SCAFLD_CONFIG_HOME", t.TempDir())

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	agentsPath := filepath.Join(root, "AGENTS.md")
	gitignorePath := filepath.Join(root, ".gitignore")
	firstAgents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	firstGitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	stdout := runCLI(t, []string{"init", "--root", root})
	if !strings.Contains(stdout, "already initialized") {
		t.Fatalf("second init stdout %q does not report idempotent no-op", stdout)
	}
	secondAgents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	secondGitignore, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstAgents) != string(secondAgents) {
		t.Fatalf("AGENTS.md changed on second init")
	}
	if string(firstGitignore) != string(secondGitignore) {
		t.Fatalf(".gitignore changed on second init:\nfirst:\n%s\nsecond:\n%s", firstGitignore, secondGitignore)
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
	if err := os.WriteFile(corePrompt, []byte("obsolete core\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(projectPrompt), 0o755); err != nil {
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
	if err := os.WriteFile(agentsPath, []byte("# scafld Agent Contract\n\nobsolete body\n\n# Project Rules\n\nproject note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCLI(t, []string{"update", "--root", root})
	data, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "obsolete body") || !strings.Contains(text, "scafld Agent Contract") || !strings.Contains(text, "project note") {
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
	observations := `Observations:
- path
  - Result: clean
  - Anchor: spec_gap:Scope
  - Note: Generated spec path exists and declared command is scoped.
- command
  - Result: clean
  - Anchor: spec_gap:Acceptance
  - Note: go version is available from the test shell.
- scope
  - Result: n/a
  - Anchor: spec_gap:Scope
  - Note: Fixture task has no migration or cutover claim.
- timing
  - Result: clean
  - Anchor: spec_gap:Phases
  - Note: Criterion can run after planning because it targets the installed go tool.
- rollback
  - Result: n/a
  - Anchor: spec_gap:Rollback
  - Note: Fixture task makes no runtime changes.
- design
  - Result: clean
  - Anchor: spec_gap:Summary
  - Note: Fixture exercises the harden lifecycle without adding compatibility behavior.
`
	observationStart := strings.Index(text, "Observations:\n")
	if observationStart < 0 {
		t.Fatalf("harden spec missing Observations block:\n%s", text)
	}
	planningStart := strings.Index(text[observationStart:], "## Planning Log")
	if planningStart < 0 {
		t.Fatalf("harden spec missing planning log:\n%s", text)
	}
	text = text[:observationStart] + observations + "\n" + text[observationStart+planningStart:]
	if err := os.WriteFile(specPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
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

func TestRunHardenProviderLocalMarksPassed(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runCLI(t, []string{"init", "--root", root})
	runCLI(t, []string{"plan", "--root", root, "provider-harden-task", "--command", "go version"})

	stdout := runCLI(t, []string{"harden", "--root", root, "provider-harden-task", "--provider", "local"})
	if !strings.Contains(stdout, "harden pass: provider-harden-task") || !strings.Contains(stdout, "next: scafld approve provider-harden-task") {
		t.Fatalf("provider harden stdout %q", stdout)
	}
	specPath := filepath.Join(root, ".scafld", "specs", "drafts", "provider-harden-task.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "harden_status: passed") ||
		!strings.Contains(text, "Verdict: pass") ||
		!strings.Contains(text, "Provider: local") ||
		!strings.Contains(text, "- design") {
		t.Fatalf("provider harden was not recorded:\n%s", text)
	}
}

func TestRunLifecycleMovesSpecsByState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "lifecycle-task", "--command", "true"})
	draftPath := filepath.Join(root, ".scafld", "specs", "drafts", "lifecycle-task.md")
	approvedPath := filepath.Join(root, ".scafld", "specs", "approved", "lifecycle-task.md")
	activePath := filepath.Join(root, ".scafld", "specs", "active", "lifecycle-task.md")
	if _, err := os.Stat(draftPath); err != nil {
		t.Fatalf("draft missing: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"build", "--root", root, "lifecycle-task"}, &stdout, &stderr)
	if code == ExitSuccess {
		t.Fatalf("build before approve succeeded unexpectedly: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	runCLI(t, []string{"approve", "--root", root, "lifecycle-task"})
	if _, err := os.Stat(draftPath); !os.IsNotExist(err) {
		t.Fatalf("draft path should move after approve: %v", err)
	}
	if _, err := os.Stat(approvedPath); err != nil {
		t.Fatalf("approved path missing: %v", err)
	}

	runCLI(t, []string{"build", "--root", root, "lifecycle-task"})
	if _, err := os.Stat(approvedPath); !os.IsNotExist(err) {
		t.Fatalf("approved path should move after build: %v", err)
	}
	if _, err := os.Stat(activePath); err != nil {
		t.Fatalf("active path missing: %v", err)
	}
	runCLI(t, []string{"build", "--root", root, "lifecycle-task"})

	command := `printf '{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"diff","attack":"scan","result":"clean"}],"budget":{"actual_attack_angles":1}}'`
	runCLI(t, []string{"review", "--root", root, "lifecycle-task", "--provider", "command", "--provider-command", command})
	runCLI(t, []string{"complete", "--root", root, "lifecycle-task"})
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Fatalf("active path should move after complete: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(root, ".scafld", "specs", "archive", "*", "lifecycle-task.md"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("archive match = %v err=%v, want one archived spec", matches, err)
	}
}

func TestRunAdapterRendersTriggerPacket(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "adapter-task", "--command", "true"})
	runCLI(t, []string{"approve", "--root", root, "adapter-task"})
	runCLI(t, []string{"build", "--root", root, "adapter-task"})

	stdout := runCLI(t, []string{"adapter", "--root", root, "codex", "build", "adapter-task"})
	for _, want := range []string{
		"# scafld Adapter Packet",
		"Provider: codex",
		"Mode: build",
		"Task: adapter-task",
		"Next action: read_handoff",
		"Command: scafld handoff adapter-task",
		"After command: scafld build adapter-task",
		"## Handoff",
		"## Build Step",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("adapter output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunBuildUsesExecutionConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	shimDir := filepath.Join(root, "tool-shims")
	if err := os.WriteFile(filepath.Join(root, ".scafld", "config.yaml"), []byte("execution:\n  path_prepend:\n    - "+shimDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCLI(t, []string{"plan", "--root", root, "env-task", "--command", `printf '%s' "$PATH" > path.txt`})
	runCLI(t, []string{"approve", "--root", root, "env-task"})
	runCLI(t, []string{"build", "--root", root, "env-task"})
	runCLI(t, []string{"build", "--root", root, "env-task"})
	data, err := os.ReadFile(filepath.Join(root, "path.txt"))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := shimDir + string(os.PathListSeparator)
	if !strings.HasPrefix(string(data), wantPrefix) {
		t.Fatalf("PATH = %q, want prefix %q", string(data), wantPrefix)
	}
}

func TestRunBuildUsesDetectedRubyToolchainShims(t *testing.T) {
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("PATH", "/bin"+string(os.PathListSeparator)+"/usr/bin")

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", ".ruby-version"), []byte("3.4.5\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runCLI(t, []string{"plan", "--root", root, "ruby-env-task", "--command", `printf '%s' "$PATH" > path.txt`})
	runCLI(t, []string{"approve", "--root", root, "ruby-env-task"})
	runCLI(t, []string{"build", "--root", root, "ruby-env-task"})
	runCLI(t, []string{"build", "--root", root, "ruby-env-task"})
	data, err := os.ReadFile(filepath.Join(root, "path.txt"))
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := filepath.Join(os.Getenv("HOME"), ".rbenv", "shims") + string(os.PathListSeparator)
	if !strings.HasPrefix(string(data), wantPrefix) {
		t.Fatalf("PATH = %q, want detected rbenv prefix %q", string(data), wantPrefix)
	}
}

func TestRunReviewSurfacesFindingsInReviewStatusAndHandoff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "review-task", "--command", "true"})
	runCLI(t, []string{"approve", "--root", root, "review-task"})
	runCLI(t, []string{"build", "--root", root, "review-task"})
	runCLI(t, []string{"build", "--root", root, "review-task"})
	command := `printf '{"verdict":"fail","mode":"discover","summary":"bug found","findings":[{"id":"f1","severity":"high","blocks_completion":true,"location":{"path":"file.go"},"evidence":"bug","impact":"breaks behavior","validation":"rerun tests","summary":"bug"}],"attack_log":[{"target":"diff","attack":"scan","result":"finding"}],"budget":{"actual_findings":1,"actual_attack_angles":1}}'`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"review", "--root", root, "review-task", "--provider", "command", "--provider-command", command}, &stdout, &stderr)
	if code != ExitReview {
		t.Fatalf("review exit = %d, want %d; stderr=%q stdout=%q", code, ExitReview, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "findings:") || !strings.Contains(stdout.String(), "bug") || !strings.Contains(stdout.String(), "next: scafld handoff review-task") {
		t.Fatalf("review output hides findings:\n%s", stdout.String())
	}
	status := runCLI(t, []string{"status", "--root", root, "review-task"})
	if !strings.Contains(status, "review: fail") || !strings.Contains(status, "bug") {
		t.Fatalf("status output hides review findings:\n%s", status)
	}
	handoff := runCLI(t, []string{"handoff", "--root", root, "review-task"})
	if !strings.Contains(handoff, "## Review Dossier") || !strings.Contains(handoff, "bug") {
		t.Fatalf("handoff hides review findings:\n%s", handoff)
	}
}

func TestRunReviewHumanReviewedOverrideCompletes(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "human-review-task", "--command", "true"})
	runCLI(t, []string{"approve", "--root", root, "human-review-task"})
	runCLI(t, []string{"build", "--root", root, "human-review-task"})
	runCLI(t, []string{"build", "--root", root, "human-review-task"})
	stdout := runCLI(t, []string{"review", "--root", root, "human-review-task", "--human-reviewed", "--reason", "operator reviewed PR 123"})
	if !strings.Contains(stdout, "review verdict: pass") || !strings.Contains(stdout, "next: scafld complete human-review-task") {
		t.Fatalf("human review output = %q", stdout)
	}
	runCLI(t, []string{"complete", "--root", root, "human-review-task"})
	sessionPath := filepath.Join(root, ".scafld", "runs", "human-review-task", "session.json")
	sessionData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sessionData), `"type": "review_override"`) || !strings.Contains(string(sessionData), "operator reviewed PR 123") {
		t.Fatalf("session missing audited override:\n%s", sessionData)
	}
}

func TestRunReviewPrintContextDoesNotInvokeProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	initGitWorkspace(t, root)
	runCLI(t, []string{"init", "--root", root, "--no-agent-docs"})
	runCLI(t, []string{"plan", "--root", root, "context-task", "--command", "true"})
	runCLI(t, []string{"approve", "--root", root, "context-task"})
	runCLI(t, []string{"build", "--root", root, "context-task"})
	runCLI(t, []string{"build", "--root", root, "context-task"})
	stdout := runCLI(t, []string{"review", "--root", root, "context-task", "--print-context", "--provider", "command", "--provider-command", `printf 'should-not-run'`})
	if !strings.Contains(stdout, "Review Context Packet") || strings.Contains(stdout, "should-not-run") {
		t.Fatalf("print context output = %q", stdout)
	}
	for _, want := range []string{"Max findings: 12", "Minimum attack angles: 6", "Review depth: standard", "Rerun policy: verify_open_blockers"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("print context missing configured budget %q:\n%s", want, stdout)
		}
	}
}

func TestReviewHelpIncludesContextFlags(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"review", "--help"}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("review help exit = %d stderr=%q", code, stderr.String())
	}
	for _, want := range []string{"--print-context", "--review-scope", "--provider", "--model", "--human-reviewed"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("review help missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestExecCommandIsNotPublicSurface(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "task"}, &stdout, &stderr)
	if code != ExitInvalid {
		t.Fatalf("exec exit = %d, want %d; stdout=%q stderr=%q", code, ExitInvalid, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown command "exec"`) {
		t.Fatalf("exec stderr = %q", stderr.String())
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

func initGitWorkspace(t *testing.T, root string) {
	t.Helper()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
}

func TestCancelledErrorsUseCancelledExitCode(t *testing.T) {
	t.Parallel()

	var stderr bytes.Buffer
	code := failOut(&stderr, context.Canceled, ExitGeneric, false)
	if code != ExitCancelled {
		t.Fatalf("exit = %d, want %d", code, ExitCancelled)
	}
}

func TestRunInitCIOptInControlsVerifyWorkflow(t *testing.T) {
	t.Setenv("SCAFLD_CONFIG_HOME", t.TempDir())

	// Default init is local-only: no CI merge-gate workflow, and the message names the opt-in.
	localRoot := t.TempDir()
	out := runCLI(t, []string{"init", "--root", localRoot, "--no-agent-docs"})
	if _, err := os.Stat(filepath.Join(localRoot, ".github", "workflows", "scafld-verify.yml")); !os.IsNotExist(err) {
		t.Fatalf("default init wrote the verify workflow (stat err=%v), want local-only", err)
	}
	if !strings.Contains(out, "not installed") || !strings.Contains(out, "scafld init --ci") {
		t.Fatalf("default init message did not name the CI opt-in:\n%s", out)
	}

	// scafld init --ci opts into the workflow and confirms it in the message.
	ciRoot := t.TempDir()
	out = runCLI(t, []string{"init", "--root", ciRoot, "--no-agent-docs", "--ci"})
	if _, err := os.Stat(filepath.Join(ciRoot, ".github", "workflows", "scafld-verify.yml")); err != nil {
		t.Fatalf("scafld init --ci did not write the verify workflow: %v", err)
	}
	if !strings.Contains(out, "installed .github/workflows/scafld-verify.yml") {
		t.Fatalf("init --ci message did not confirm install:\n%s", out)
	}
}
