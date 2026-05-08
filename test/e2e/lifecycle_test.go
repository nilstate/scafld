package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLifecycleJSONContractsAgentSurfaceFailCancelReviewProviderMutationGuard(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	assertSnakeEnvelope(t, run(t, bin, "init", "--root", root, "--json"), "root")
	initGitWorkspace(t, root)
	assertSnakeEnvelope(t, run(t, bin, "plan", "--root", root, "lifecycle-task", "--title", "Lifecycle task", "--command", "test -f .scafld/config.yaml", "--json"), "task_id")
	assertSnakeEnvelope(t, run(t, bin, "validate", "--root", root, "lifecycle-task", "--json"), "valid")
	assertSnakeEnvelope(t, run(t, bin, "approve", "--root", root, "lifecycle-task", "--json"), "status")
	assertSnakeEnvelope(t, run(t, bin, "build", "--root", root, "lifecycle-task", "--json"), "passed")
	assertSnakeEnvelope(t, run(t, bin, "list", "--root", root, "--json"), "task_id")
	assertSnakeEnvelope(t, run(t, bin, "review", "--root", root, "--provider", "command", "--provider-command", `printf '{"verdict":"pass","findings":[]}'`, "lifecycle-task", "--json"), "verdict")
	assertSnakeEnvelope(t, run(t, bin, "complete", "--root", root, "lifecycle-task", "--json"), "current_state")
	assertSnakeEnvelope(t, run(t, bin, "status", "--root", root, "lifecycle-task", "--json"), "session_ok")
	assertSnakeEnvelope(t, run(t, bin, "report", "--root", root, "--json"), "by_status")
	if _, err := os.Stat(filepath.Join(root, ".scafld", "runs", "lifecycle-task", "session.json")); err != nil {
		t.Fatal(err)
	}
}

func TestFailCancel(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	run(t, bin, "plan", "--root", root, "cancel-task", "--command", "true")
	run(t, bin, "cancel", "--root", root, "cancel-task", "--reason", "test")
	run(t, bin, "plan", "--root", root, "fail-task", "--command", "false")
	run(t, bin, "fail", "--root", root, "fail-task", "--reason", "test failure")
}

func TestReviewCommandProviderBlockingFindingExitsReviewFailure(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	initGitWorkspace(t, root)
	run(t, bin, "plan", "--root", root, "provider-task", "--command", "true")
	run(t, bin, "approve", "--root", root, "provider-task")
	run(t, bin, "build", "--root", root, "provider-task")
	cmd := exec.Command(
		bin,
		"review",
		"--root",
		root,
		"--provider-command",
		`grep 'provider-task' >/dev/null && printf '{"type":"finding","severity":"blocking","summary":"bug"}\n'`,
		"provider-task",
	)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err == nil {
		t.Fatal("blocking finding should exit with review failure")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 4 {
		t.Fatalf("review exit = %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "review verdict: fail") {
		t.Fatalf("stdout %q does not contain fail verdict", out.String())
	}
	for _, want := range []string{
		"findings:",
		"[blocking] finding-1: bug",
		"next: scafld handoff provider-task",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, out.String())
		}
	}
	status := string(run(t, bin, "status", "--root", root, "provider-task"))
	if !strings.Contains(status, "review: fail") || !strings.Contains(status, "[blocking] finding-1: bug") {
		t.Fatalf("status did not surface review findings:\n%s", status)
	}
	handoff := string(run(t, bin, "handoff", "--root", root, "provider-task"))
	if !strings.Contains(handoff, "bug") || !strings.Contains(handoff, "blocking") {
		t.Fatalf("handoff did not surface review findings:\n%s", handoff)
	}
}

func TestReviewProviderMutationGuardFailsReview(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	initGitWorkspace(t, root)
	run(t, bin, "plan", "--root", root, "mutation-task", "--command", "true")
	run(t, bin, "approve", "--root", root, "mutation-task")
	run(t, bin, "build", "--root", root, "mutation-task")
	cmd := exec.Command(
		bin,
		"review",
		"--root",
		root,
		"--provider-command",
		`touch MUTATED && printf '{"type":"verdict","verdict":"pass"}\n'`,
		"mutation-task",
	)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err == nil {
		t.Fatal("workspace mutation should exit with review failure")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 4 {
		t.Fatalf("review exit = %v\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), "review verdict: fail") {
		t.Fatalf("stdout %q does not contain mutation failure verdict", out.String())
	}
}

func TestReviewContextPreview(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	initGitWorkspace(t, root)
	run(t, bin, "plan", "--root", root, "context-preview", "--title", "Context Preview", "--command", "true")
	run(t, bin, "approve", "--root", root, "context-preview")
	run(t, bin, "build", "--root", root, "context-preview")
	out := string(run(t, bin, "review", "--root", root, "context-preview", "--print-context", "--provider", "command", "--provider-command", `printf 'should-not-run'`))
	for _, want := range []string{"Review Context Packet", "Task: context-preview", "Task Contract"} {
		if !strings.Contains(out, want) {
			t.Fatalf("context preview missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "should-not-run") {
		t.Fatalf("print-context invoked provider:\n%s", out)
	}
}

func TestHardenPreservesHumanOwnedSpecSections(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	run(t, bin, "plan", "--root", root, "harden-preserve", "--title", "Harden preserve", "--command", "true")
	specPath := filepath.Join(root, ".scafld", "specs", "drafts", "harden-preserve.md")
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	text = strings.Replace(text, "## Objectives\n\n", "## Context\n\nFiles impacted:\n- `backend/app.go` - preserve this human-owned detail\n\n```text\n## Phase 99: Not a real phase\n```\n\n## Objectives\n\n", 1)
	text = strings.Replace(text, "## Risks\n\n- none\n", "## Risks\n\n- Hardening can erase detail if section replacement is too broad\n  - Mitigation: keep targeted section writes under test\n", 1)
	if err := os.WriteFile(specPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, bin, "harden", "--root", root, "harden-preserve")
	updated, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	for _, want := range []string{
		"Files impacted:\n- `backend/app.go` - preserve this human-owned detail",
		"## Phase 99: Not a real phase",
		"Hardening can erase detail if section replacement is too broad",
		"harden_status: in_progress",
		"## Harden Rounds\n\n### round-1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("hardened spec lost %q:\n%s", want, got)
		}
	}
}

func TestInitUpdatePreservesProjectOwnedFilesAndGitignore(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	projectPrompt := filepath.Join(root, ".scafld", "prompts", "harden.md")
	if err := os.WriteFile(projectPrompt, []byte("# Project harden prompt\n\nKeep this exact prompt.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agentsPath := filepath.Join(root, "AGENTS.md")
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	agents = append(agents, []byte("\n# Project Agent Rules\n\nKeep local instructions.\n")...)
	if err := os.WriteFile(agentsPath, agents, 0o644); err != nil {
		t.Fatal(err)
	}

	run(t, bin, "init", "--root", root)
	run(t, bin, "update", "--root", root)

	prompt, err := os.ReadFile(projectPrompt)
	if err != nil {
		t.Fatal(err)
	}
	if string(prompt) != "# Project harden prompt\n\nKeep this exact prompt.\n" {
		t.Fatalf("project prompt was clobbered:\n%s", prompt)
	}
	agents, err = os.ReadFile(agentsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "# scafld Agent Contract") || !strings.Contains(string(agents), "# Project Agent Rules") {
		t.Fatalf("agent docs did not preserve scafld and project sections:\n%s", agents)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(string(gitignore), "# scafld runtime state"); count != 1 {
		t.Fatalf("scafld gitignore block count = %d:\n%s", count, gitignore)
	}
	if !strings.Contains(string(gitignore), ".scafld/runs/") || !strings.Contains(string(gitignore), "!.scafld/specs/**") {
		t.Fatalf("gitignore missing scafld tracked/runtime rules:\n%s", gitignore)
	}
}

func TestExitCodeTable(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	cmd := exec.Command(bin, "missing")
	if err := cmd.Run(); err == nil {
		t.Fatal("unknown command should fail")
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 2 {
		t.Fatalf("exit = %v", err)
	}
}

func testBinary(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("SCAFLD_E2E_BINARY"); bin != "" {
		if !filepath.IsAbs(bin) {
			return filepath.Join("..", "..", bin)
		}
		return bin
	}
	bin := filepath.Join(t.TempDir(), "scafld")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/scafld")
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build e2e binary: %v\n%s", err, out)
	}
	return bin
}

func initGitWorkspace(t *testing.T, root string) {
	t.Helper()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
}

func run(t *testing.T, bin string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", bin, strings.Join(args, " "), err, out.String(), errOut.String())
	}
	return out.Bytes()
}

func assertSnakeEnvelope(t *testing.T, data []byte, requiredResultKey string) {
	t.Helper()
	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, data)
	}
	if envelope["ok"] != true {
		t.Fatalf("envelope not ok: %s", data)
	}
	assertSnakeKeys(t, envelope, "$")
	result, ok := envelope["result"].(map[string]any)
	if !ok {
		if records, recordsOK := envelope["result"].([]any); recordsOK {
			if len(records) == 0 {
				t.Fatalf("result list is empty: %s", data)
			}
			result, ok = records[0].(map[string]any)
		}
	}
	if !ok {
		t.Fatalf("result is not an object or non-empty object list: %s", data)
	}
	if _, ok := result[requiredResultKey]; !ok {
		t.Fatalf("result missing %q: %s", requiredResultKey, data)
	}
}

func assertSnakeKeys(t *testing.T, value any, path string) {
	t.Helper()
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			for _, r := range key {
				if r >= 'A' && r <= 'Z' {
					t.Fatalf("non-snake JSON key at %s.%s", path, key)
				}
			}
			assertSnakeKeys(t, nested, path+"."+key)
		}
	case []any:
		for i, nested := range typed {
			assertSnakeKeys(t, nested, path+"[]")
			if i > 10 {
				break
			}
		}
	}
}
