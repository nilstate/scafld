package corebundle

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRepositoryDoesNotTrackGeneratedWorkspaceCopies(t *testing.T) {
	t.Parallel()

	for _, path := range []string{".scafld/core", ".scafld/prompts", ".scafld/specs/archive", ".scafld/specs/examples"} {
		out, err := exec.Command("git", "-C", repoRoot(t), "ls-files", "--", path).CombinedOutput()
		if err != nil {
			t.Fatalf("git ls-files %s: %v\n%s", path, err, out)
		}
		if strings.TrimSpace(string(out)) != "" {
			t.Fatalf("generated workspace files are tracked under %s:\n%s", path, out)
		}
	}
}

func TestUpdateRefreshesUnmodifiedProjectPromptsFromManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(root, ".scafld", "prompts", "review.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	oldPrompt := []byte("# old default review prompt\n")
	if err := os.WriteFile(promptPath, oldPrompt, 0o644); err != nil {
		t.Fatal(err)
	}
	writeTestPromptManifest(t, root, map[string]string{"review.md": sha256Hex(oldPrompt)})

	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(result.Updated, ".scafld/prompts/review.md") {
		t.Fatalf("updated = %v, want review prompt refreshed", result.Updated)
	}
	got, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := assets.ReadFile("assets/core/prompts/review.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("review prompt not refreshed from bundle")
	}
}

func TestUpdateSkipsFilesThatAlreadyMatchBundle(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 0 || len(result.Created) != 0 {
		t.Fatalf("update should be content-idempotent, got created=%v updated=%v", result.Created, result.Updated)
	}
}

func TestInitDoesNotInstallLifecycleHelperScripts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := Init(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range lifecycleHelperScripts() {
		if containsPath(result.Created, rel) {
			t.Fatalf("default init created optional lifecycle helper %s", rel)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("default init lifecycle helper %s stat err=%v, want missing", rel, err)
		}
	}
}

func TestUpdateInstallsOptionalLifecycleHelperScripts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range lifecycleHelperScripts() {
		if !containsPath(result.Created, rel) {
			t.Fatalf("update created=%v, want optional lifecycle helper %s", result.Created, rel)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s missing after update: %v", rel, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("%s should be executable after update: %v", rel, info.Mode())
		}
	}
}

func TestUpdateLeavesCurrentProjectConfigShapeAlone(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	configPath := filepath.Join(root, ".scafld", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	config := []byte(`
version: "1.0"
invariants:
  canonical:
    domain_boundaries: "Respect boundaries."
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if containsPath(result.Updated, ".scafld/config.yaml") {
		t.Fatalf("updated = %v, want current config left alone", result.Updated)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, config) {
		t.Fatalf("current config changed:\n%s", got)
	}
}

func TestInitCreatesSparseProjectConfigAndFullCoreExample(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	project, err := os.ReadFile(filepath.Join(root, ".scafld", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(project), "Keep this file sparse") || strings.Contains(string(project), "adversarial_passes:") {
		t.Fatalf("project config should be sparse:\n%s", project)
	}
	core, err := os.ReadFile(filepath.Join(root, ".scafld", "core", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"review:", "adversarial_passes:", "harden:"} {
		if !strings.Contains(string(core), want) {
			t.Fatalf("core config example missing %q:\n%s", want, core)
		}
	}
}

func TestUpdateSkipsCustomizedProjectPrompt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(root, ".scafld", "prompts", "review.md")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	custom := []byte("# custom review prompt\n")
	if err := os.WriteFile(promptPath, custom, 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPath(result.Skipped, ".scafld/prompts/review.md") {
		t.Fatalf("skipped = %v, want customized review prompt skipped", result.Skipped)
	}
	got, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, custom) {
		t.Fatalf("custom prompt was overwritten:\n%s", got)
	}
}

func TestInitGitignoreCreatesScafldRules(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := InitGitignore(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 1 || result.Created[0] != ".gitignore" {
		t.Fatalf("created = %v, want .gitignore", result.Created)
	}
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"# scafld runtime state",
		"!.scafld/specs/**",
		".scafld/config.local.yaml",
		".scafld/core/",
		".scafld/prompts/.manifest.json",
		".scafld/runs/",
		"!.scafld/receipts/*.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf(".gitignore missing %q:\n%s", want, text)
		}
	}
}

func writeTestPromptManifest(t *testing.T, root string, prompts map[string]string) {
	t.Helper()

	manifest := promptManifest{Version: 1, Prompts: prompts}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".scafld", "prompts", ".manifest.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func lifecycleHelperScripts() []string {
	out := make([]string, 0, len(lifecycleHelperScriptPaths))
	for _, rel := range lifecycleHelperScriptPaths {
		out = append(out, ".scafld/core/"+rel)
	}
	return out
}

func TestInitGitignoreIsIdempotentAndOverridesBroadScafldIgnore(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n.scafld/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := InitGitignore(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != ".gitignore" {
		t.Fatalf("updated = %v, want .gitignore", result.Updated)
	}
	first, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	result, err = InitGitignore(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != ".gitignore" {
		t.Fatalf("second init skipped = %v, want .gitignore", result.Skipped)
	}
	second, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf(".gitignore changed on second init:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if count := strings.Count(string(second), "# scafld runtime state"); count != 1 {
		t.Fatalf("scafld block count = %d, want 1:\n%s", count, second)
	}
	if err := exec.Command("git", "-C", root, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	assertGitIgnore(t, root, ".scafld/runs/task/session.json", true)
	assertGitIgnore(t, root, ".scafld/receipts/task.log", true)
	assertGitIgnore(t, root, ".scafld/core/config.yaml", true)
	assertGitIgnore(t, root, ".scafld/prompts/.manifest.json", true)
	assertGitIgnore(t, root, ".scafld/config.local.yaml", true)
	assertGitIgnore(t, root, ".scafld/receipts/task.json", false)
	assertGitIgnore(t, root, ".scafld/prompts/harden.md", false)
	assertGitIgnore(t, root, ".scafld/specs/drafts/task.md", false)
	assertGitIgnore(t, root, ".scafld/config.yaml", false)
}

func assertGitIgnore(t *testing.T, root string, rel string, wantIgnored bool) {
	t.Helper()

	cmd := exec.Command("git", "-C", root, "check-ignore", "--quiet", rel)
	err := cmd.Run()
	gotIgnored := err == nil
	if gotIgnored != wantIgnored {
		t.Fatalf("git ignore for %s = %v, want %v", rel, gotIgnored, wantIgnored)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", file)
		}
		dir = parent
	}
}
