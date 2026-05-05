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
}

func TestReviewProviderMutationGuardFailsReview(t *testing.T) {
	t.Parallel()

	bin := testBinary(t)
	root := t.TempDir()
	run(t, bin, "init", "--root", root)
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
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
