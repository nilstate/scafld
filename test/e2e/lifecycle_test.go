package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func reviewCommandPrintf(payload string) string {
	return "printf '%s\\n' '" + payload + "'"
}

func wrappedReviewDossierJSON(dossier string) string {
	return `{"type":"dossier","dossier":` + dossier + `}`
}

func passingReviewDossierJSON(mode string, summary string, target string, attackCount int) string {
	return fmt.Sprintf(`{"verdict":"pass","mode":"%s","summary":"%s","findings":[],"attack_log":[%s],"budget":{"actual_attack_angles":%d}}`, mode, summary, reviewAttackLogJSON(target, "clean", attackCount), attackCount)
}

func failingReviewDossierJSON(summary string, id string, evidence string, impact string, path string, attackCount int) string {
	return fmt.Sprintf(`{"verdict":"fail","mode":"discover","summary":"%s","findings":[{"id":"%s","severity":"high","blocks_completion":true,"location":{"path":"%s"},"evidence":"%s","impact":"%s","validation":"rerun tests","summary":"%s"}],"attack_log":[%s],"budget":{"actual_findings":1,"actual_attack_angles":%d}}`, summary, id, path, evidence, impact, summary, reviewAttackLogJSON("diff", "finding", attackCount), attackCount)
}

func reviewAttackLogJSON(target string, result string, attackCount int) string {
	entries := make([]string, 0, attackCount)
	for i := 0; i < attackCount; i++ {
		entries = append(entries, fmt.Sprintf(`{"target":"%s","attack":"scan-%d","result":"%s"}`, target, i+1, result))
	}
	return strings.Join(entries, ",")
}

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
	assertSnakeEnvelope(t, run(t, bin, "build", "--root", root, "lifecycle-task", "--json"), "passed")
	assertSnakeEnvelope(t, run(t, bin, "list", "--root", root, "--json"), "task_id")
	assertSnakeEnvelope(t, run(t, bin, "review", "--root", root, "--provider", "command", "--provider-command", reviewCommandPrintf(passingReviewDossierJSON("discover", "clean", "diff", 6)), "lifecycle-task", "--json"), "verdict")
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

func TestDevWrapperPreservesScafldExitCode(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("dev wrapper is a POSIX shell script")
	}

	wrapper, err := filepath.Abs(filepath.Join("..", "..", "bin", "scafld"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(wrapper, "exec", "task")
	cmd.Dir = t.TempDir()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 2 {
		t.Fatalf("wrapper exit = %v, want exit 2\nstdout:\n%s\nstderr:\n%s", err, out.String(), errOut.String())
	}
	if !strings.Contains(errOut.String(), `unknown command "exec"`) || strings.Contains(errOut.String(), "exit status") {
		t.Fatalf("wrapper stderr did not preserve scafld output:\n%s", errOut.String())
	}
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
	run(t, bin, "build", "--root", root, "provider-task")
	cmd := exec.Command(
		bin,
		"review",
		"--root",
		root,
		"--provider-command",
		"grep 'provider-task' >/dev/null && "+reviewCommandPrintf(wrappedReviewDossierJSON(failingReviewDossierJSON("bug found", "bug", "bug", "breaks behavior", "file.go", 6))),
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
		"[high/blocks completion] bug: bug",
		"next: scafld handoff provider-task",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stdout missing %q:\n%s", want, out.String())
		}
	}
	status := string(run(t, bin, "status", "--root", root, "provider-task"))
	if !strings.Contains(status, "review: fail") || !strings.Contains(status, "[high/blocks completion] bug: bug") {
		t.Fatalf("status did not surface review findings:\n%s", status)
	}
	handoff := string(run(t, bin, "handoff", "--root", root, "provider-task"))
	if !strings.Contains(handoff, "bug") || !strings.Contains(handoff, "blocks completion") {
		t.Fatalf("handoff did not surface review findings:\n%s", handoff)
	}
}

func TestFailedReviewRequiresRepairBuildBeforeCompletion(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	initGitWorkspace(t, root)
	run(t, bin, "plan", "--root", root, "repair-loop", "--title", "Repair Loop", "--command", "true")
	run(t, bin, "approve", "--root", root, "repair-loop")
	run(t, bin, "build", "--root", root, "repair-loop")
	run(t, bin, "build", "--root", root, "repair-loop")

	failedReview := runExpectExit(t, 4, bin,
		"review",
		"--root", root,
		"--provider-command",
		reviewCommandPrintf(wrappedReviewDossierJSON(failingReviewDossierJSON("repair required", "loop-bug", "missing repair", "completion would ship the bug", "fixed.txt", 6))),
		"repair-loop",
	)
	for _, want := range []string{"review verdict: fail", "repair required", "next: scafld handoff repair-loop"} {
		if !strings.Contains(failedReview.Stdout, want) {
			t.Fatalf("failed review missing %q:\nstdout:\n%s\nstderr:\n%s", want, failedReview.Stdout, failedReview.Stderr)
		}
	}

	blockedComplete := runExpectFailure(t, bin, "complete", "--root", root, "repair-loop")
	if !strings.Contains(blockedComplete.Stderr, "review gate has not passed") {
		t.Fatalf("premature complete did not explain review gate:\nstdout:\n%s\nstderr:\n%s", blockedComplete.Stdout, blockedComplete.Stderr)
	}
	status := string(run(t, bin, "status", "--root", root, "repair-loop"))
	if !strings.Contains(status, "next: scafld handoff repair-loop") || !strings.Contains(status, "review: fail") {
		t.Fatalf("status did not point back to repair:\n%s", status)
	}
	handoff := string(run(t, bin, "handoff", "--root", root, "repair-loop"))
	for _, want := range []string{"Repair focus:", "After repair, run `scafld build repair-loop`", "Then run `scafld review repair-loop`"} {
		if !strings.Contains(handoff, want) {
			t.Fatalf("handoff missing %q:\n%s", want, handoff)
		}
	}

	run(t, bin, "build", "--root", root, "repair-loop")
	run(t, bin,
		"review",
		"--root", root,
		"--mode", "verify",
		"--provider-command",
		reviewCommandPrintf(wrappedReviewDossierJSON(passingReviewDossierJSON("verify", "repair verified", "repair", 6))),
		"repair-loop",
	)
	run(t, bin, "complete", "--root", root, "repair-loop")
	finalStatus := string(run(t, bin, "status", "--root", root, "repair-loop"))
	if !strings.Contains(finalStatus, "repair-loop: completed") {
		t.Fatalf("task did not complete after repair loop:\n%s", finalStatus)
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
	run(t, bin, "build", "--root", root, "mutation-task")
	cmd := exec.Command(
		bin,
		"review",
		"--root",
		root,
		"--provider-command",
		"touch MUTATED && "+reviewCommandPrintf(wrappedReviewDossierJSON(passingReviewDossierJSON("discover", "clean", "diff", 6))),
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

func TestCompleteAcceptsCommitOnlyAfterMaterialSealedReview(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	initGitWorkspace(t, root)
	writeFile(t, root, "app.txt", "before\n")
	gitCommitAll(t, root, "initial")
	run(t, bin, "init", "--root", root, "--no-agent-docs")
	run(t, bin, "plan", "--root", root, "commit-after-review", "--title", "Commit After Review", "--command", "true")
	run(t, bin, "approve", "--root", root, "commit-after-review")
	run(t, bin, "build", "--root", root, "commit-after-review")
	writeFile(t, root, "app.txt", "after\n")
	run(t, bin, "build", "--root", root, "commit-after-review")
	run(t, bin, "review", "--root", root, "commit-after-review", "--provider", "command", "--provider-command", reviewCommandPrintf(passingReviewDossierJSON("discover", "clean", "app.txt", 6)))
	gitCommitPaths(t, root, "commit reviewed material", "app.txt")
	out := string(run(t, bin, "complete", "--root", root, "commit-after-review"))
	if !strings.Contains(out, "complete: commit-after-review") {
		t.Fatalf("complete after commit-only change failed to report completion:\n%s", out)
	}
}

func TestCompleteAcceptsAmbientDriftOutsideMaterialScope(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	initGitWorkspace(t, root)
	writeFile(t, root, "app.txt", "before\n")
	gitCommitAll(t, root, "initial")
	run(t, bin, "init", "--root", root, "--no-agent-docs")
	run(t, bin, "plan", "--root", root, "ambient-after-review", "--title", "Ambient After Review", "--command", "true")
	run(t, bin, "approve", "--root", root, "ambient-after-review")
	run(t, bin, "build", "--root", root, "ambient-after-review")
	writeFile(t, root, "app.txt", "after\n")
	run(t, bin, "build", "--root", root, "ambient-after-review")
	run(t, bin, "review", "--root", root, "ambient-after-review", "--provider", "command", "--provider-command", reviewCommandPrintf(passingReviewDossierJSON("discover", "clean", "app.txt", 6)))
	writeFile(t, root, "adjacent.txt", "other agent work\n")
	out := string(run(t, bin, "complete", "--root", root, "ambient-after-review"))
	if !strings.Contains(out, "complete: ambient-after-review") {
		t.Fatalf("complete with ambient drift failed to report completion:\n%s", out)
	}
}

func TestCompleteRejectsChangedMaterialAfterReview(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	initGitWorkspace(t, root)
	writeFile(t, root, "app.txt", "before\n")
	gitCommitAll(t, root, "initial")
	run(t, bin, "init", "--root", root, "--no-agent-docs")
	run(t, bin, "plan", "--root", root, "material-after-review", "--title", "Material After Review", "--command", "true")
	run(t, bin, "approve", "--root", root, "material-after-review")
	run(t, bin, "build", "--root", root, "material-after-review")
	writeFile(t, root, "app.txt", "reviewed\n")
	run(t, bin, "build", "--root", root, "material-after-review")
	run(t, bin, "review", "--root", root, "material-after-review", "--provider", "command", "--provider-command", reviewCommandPrintf(passingReviewDossierJSON("discover", "clean", "app.txt", 6)))
	writeFile(t, root, "app.txt", "changed after review\n")
	result := runExpectExit(t, 3, bin, "complete", "--root", root, "material-after-review")
	if !strings.Contains(result.Stderr, "latest review is stale against current task material") {
		t.Fatalf("complete did not explain material staleness:\nstdout:\n%s\nstderr:\n%s", result.Stdout, result.Stderr)
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
	binName := "scafld"
	if runtime.GOOS == "windows" {
		binName = "scafld.exe"
	}
	bin := filepath.Join(t.TempDir(), binName)
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

func gitCommitAll(t *testing.T, root string, message string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "add", ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	gitCommit(t, root, message)
}

func gitCommitPaths(t *testing.T, root string, message string, paths ...string) {
	t.Helper()
	args := append([]string{"-C", root, "add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add paths failed: %v\n%s", err, out)
	}
	gitCommit(t, root, message)
}

func gitCommit(t *testing.T, root string, message string) {
	t.Helper()
	cmd := exec.Command("git", "-C", root, "-c", "user.email=scafld@example.invalid", "-c", "user.name=scafld tests", "commit", "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
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

type commandResult struct {
	Stdout string
	Stderr string
}

func runExpectExit(t *testing.T, wantExit int, bin string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()
	if err == nil {
		t.Fatalf("%s %s succeeded, want exit %d\nstdout:\n%s\nstderr:\n%s", bin, strings.Join(args, " "), wantExit, out.String(), errOut.String())
	}
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != wantExit {
		t.Fatalf("%s %s exit = %v, want %d\nstdout:\n%s\nstderr:\n%s", bin, strings.Join(args, " "), err, wantExit, out.String(), errOut.String())
	}
	return commandResult{Stdout: out.String(), Stderr: errOut.String()}
}

func runExpectFailure(t *testing.T, bin string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err == nil {
		t.Fatalf("%s %s succeeded, want failure\nstdout:\n%s\nstderr:\n%s", bin, strings.Join(args, " "), out.String(), errOut.String())
	}
	return commandResult{Stdout: out.String(), Stderr: errOut.String()}
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
