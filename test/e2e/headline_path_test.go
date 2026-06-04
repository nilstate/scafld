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

func TestHeadlinePathExecutesFinalizeReceiptVerify(t *testing.T) {
	bin := testBinary(t)
	root := t.TempDir()
	configHome := t.TempDir()
	fakeCodex := writeFakeCodexReviewer(t)

	runWithEnv(t, []string{"SCAFLD_CONFIG_HOME=" + configHome}, bin, "init", "--root", root, "--no-agent-docs")
	initGitWorkspace(t, root)
	runGit(t, root, "config", "user.name", "scafld")
	runGit(t, root, "config", "user.email", "scafld@example.invalid")
	writeFile(t, root, "file.txt", "base\n")
	runGit(t, root, "add", "-A")
	runGit(t, root, "commit", "-m", "base")
	writeFile(t, root, "file.txt", "changed\n")
	head := gitOutput(t, root, "rev-parse", "HEAD")

	configPath := filepath.Join(root, ".scafld", "config.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	config = append(config, []byte("\nreview:\n  external:\n    provider: \"codex\"\n    codex:\n      binary: "+quoteYAML(fakeCodex)+"\n")...)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	finalizeReq := map[string]any{
		"task_id":    "headline-path",
		"root":       root,
		"scope_hint": []string{"file.txt"},
	}
	request, err := json.Marshal(finalizeReq)
	if err != nil {
		t.Fatal(err)
	}
	finalizeOut := runWithInput(t, []string{
		"SCAFLD_CONFIG_HOME=" + configHome,
		"OPENAI_API_KEY=fake-test-key",
	}, bin, []string{"finalize", "--json", "--stdin"}, append(request, '\n'))
	var finalizeResp struct {
		OK              bool   `json:"ok"`
		Verdict         string `json:"verdict"`
		ReceiptPath     string `json:"receipt_path"`
		TaskReceiptPath string `json:"task_receipt_path"`
	}
	if err := json.Unmarshal(finalizeOut, &finalizeResp); err != nil {
		t.Fatalf("decode finalize response: %v\n%s", err, finalizeOut)
	}
	if !finalizeResp.OK || finalizeResp.Verdict != "pass" {
		t.Fatalf("finalize response = %+v\n%s", finalizeResp, finalizeOut)
	}
	if filepath.Base(finalizeResp.ReceiptPath) != "latest.json" {
		t.Fatalf("receipt_path = %q, want latest.json", finalizeResp.ReceiptPath)
	}
	for _, path := range []string{finalizeResp.ReceiptPath, finalizeResp.TaskReceiptPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("receipt file %s missing: %v", path, err)
		}
	}

	trustedKeys := filepath.Join(root, ".scafld", "trusted-keys.json")
	runWithEnv(t, []string{"SCAFLD_CONFIG_HOME=" + configHome}, bin, "verify", "--root", root, finalizeResp.ReceiptPath, "--target", head, "--trusted-keys", trustedKeys)
	runWithEnv(t, []string{"SCAFLD_CONFIG_HOME=" + configHome}, bin, "verify", "--root", root, "--target", head, "--trusted-keys", trustedKeys)
}

func writeFakeCodexReviewer(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codex")
	script := `#!/bin/sh
set -eu
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--output-last-message" ]; then
    shift
    out="$1"
  fi
  shift || true
done
if [ -z "$out" ]; then
  echo "missing --output-last-message" >&2
  exit 2
fi
printf '%s\n' '{"verdict":"pass","mode":"discover","summary":"clean","findings":[],"attack_log":[{"target":"headline path","attack":"review evidence","result":"clean"}],"budget":{"actual_attack_angles":1}}' > "$out"
`
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runWithEnv(t *testing.T, env []string, bin string, args ...string) []byte {
	t.Helper()
	return runWithInput(t, env, bin, args, nil)
}

func runWithInput(t *testing.T, env []string, bin string, args []string, input []byte) []byte {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), env...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %s failed: %v\nstdout:\n%s\nstderr:\n%s", bin, strings.Join(args, " "), err, out.String(), errOut.String())
	}
	return out.Bytes()
}

func quoteYAML(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

func gitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}
