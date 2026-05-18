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

type assetFile struct {
	data []byte
	exec bool
}

func TestEmbeddedAssetsDoNotDriftFromWorkspaceAssets(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	assertAssetTreeMatches(t, filepath.Join(root, ".scafld", "core"), filepath.Join(root, "internal", "adapters", "corebundle", "assets", "core"), ".scafld/core")
	assertAssetTreeMatches(t, filepath.Join(root, ".scafld", "prompts"), filepath.Join(root, "internal", "adapters", "corebundle", "assets", "prompts"), ".scafld/prompts")
}

func TestUpdateRefreshesUnmodifiedProjectPromptsFromManifest(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := Init(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	promptPath := filepath.Join(root, ".scafld", "prompts", "review.md")
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
	want, err := assets.ReadFile("assets/prompts/review.md")
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
	result, err := Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 0 || len(result.Created) != 0 {
		t.Fatalf("update should be content-idempotent, got created=%v updated=%v", result.Created, result.Updated)
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
		".scafld/runs/",
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
	assertGitIgnore(t, root, ".scafld/config.local.yaml", true)
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

func assertAssetTreeMatches(t *testing.T, canonicalRoot string, embeddedRoot string, label string) {
	t.Helper()

	canonical := collectAssetTree(t, canonicalRoot)
	embedded := collectAssetTree(t, embeddedRoot)

	for rel, want := range canonical {
		got, ok := embedded[rel]
		if !ok {
			t.Fatalf("%s drift: embedded bundle is missing %s; sync %s into %s", label, rel, canonicalRoot, embeddedRoot)
		}
		if !bytes.Equal(got.data, want.data) {
			t.Fatalf("%s drift: %s differs between %s and %s", label, rel, canonicalRoot, embeddedRoot)
		}
		if got.exec != want.exec {
			t.Fatalf("%s drift: %s executable bit differs between %s and %s", label, rel, canonicalRoot, embeddedRoot)
		}
	}

	for rel := range embedded {
		if _, ok := canonical[rel]; !ok {
			t.Fatalf("%s drift: embedded bundle has extra file %s; remove it or add the canonical file under %s", label, rel, canonicalRoot)
		}
	}
}

func collectAssetTree(t *testing.T, root string) map[string]assetFile {
	t.Helper()

	files := make(map[string]assetFile)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("stat asset root %s: %v", root, err)
	}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == ".manifest.json" {
			return nil
		}
		files[rel] = assetFile{
			data: data,
			exec: info.Mode()&0o111 != 0,
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk asset root %s: %v", root, err)
	}
	return files
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
