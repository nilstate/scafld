package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangedFilesFingerprintsContentChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	path := filepath.Join(root, "file.txt")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := (Adapter{Root: root}).ChangedFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := (Adapter{Root: root}).ChangedFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 || len(after) != 1 || before[0] == after[0] {
		t.Fatalf("before=%+v after=%+v", before, after)
	}
}

func TestChangedFilesFailsClosedWhenGitStatusFails(t *testing.T) {
	t.Parallel()

	_, err := (Adapter{Root: t.TempDir()}).ChangedFiles(context.Background())
	if err == nil {
		t.Fatal("ChangedFiles returned nil error outside a git worktree")
	}
}

func TestChangedFilesIgnoresScafldRuntimeState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	for rel, data := range map[string][]byte{
		".scafld/runs/task/session.json": []byte("{}\n"),
		".scafld/specs/drafts/task.md":   []byte("# task\n"),
		".scafld/config.yaml":            []byte("review: {}\n"),
		"api/handler.go":                 []byte("package api\n"),
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	changed, err := (Adapter{Root: root}).ChangedFiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(changed, "\n")
	if strings.Contains(joined, ".scafld/runs") {
		t.Fatalf("runtime path leaked into changed files:\n%s", joined)
	}
	for _, kept := range []string{".scafld/config.yaml", ".scafld/specs/drafts/task.md", "api/handler.go"} {
		if !strings.Contains(joined, kept) {
			t.Fatalf("expected %q in changed files:\n%s", kept, joined)
		}
	}
}

func TestChangedFilesFingerprintsDirectoriesWithoutDeletedFalsePositive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	dir := filepath.Join(root, "cloud")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := Adapter{Root: root}
	before := adapter.fingerprint(context.Background(), " M", "cloud")
	if strings.Contains(before, "deleted cloud") {
		t.Fatalf("directory fingerprint reported deleted: %q", before)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := adapter.fingerprint(context.Background(), " M", "cloud")
	if before == after {
		t.Fatalf("directory content mutation did not change fingerprint: %q", before)
	}
}

func TestChangedFilesFingerprintsNestedGitWorktreesWithoutDeletedFalsePositive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "cloud")
	if out, err := exec.Command("git", "init", sub).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", sub, "-c", "user.name=scafld", "-c", "user.email=scafld@example.invalid", "add", "README.md")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", sub, "-c", "user.name=scafld", "-c", "user.email=scafld@example.invalid", "commit", "-m", "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	adapter := Adapter{Root: root}
	before := adapter.fingerprint(context.Background(), " M", "cloud")
	if strings.Contains(before, "deleted cloud") {
		t.Fatalf("nested git worktree fingerprint reported deleted: %q", before)
	}
	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := adapter.fingerprint(context.Background(), " M", "cloud")
	if before == after {
		t.Fatalf("nested git worktree mutation did not change fingerprint: %q", before)
	}
}

func TestChangedFilesFingerprintsNestedGitWorktreeDirtyContentChanges(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	sub := filepath.Join(root, "cloud")
	if out, err := exec.Command("git", "init", sub).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("base"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", sub, "-c", "user.name=scafld", "-c", "user.email=scafld@example.invalid", "add", "README.md")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "-C", sub, "-c", "user.name=scafld", "-c", "user.email=scafld@example.invalid", "commit", "-m", "init")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("dirty one"), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := Adapter{Root: root}
	before := adapter.fingerprint(context.Background(), " M", "cloud")
	if err := os.WriteFile(filepath.Join(sub, "README.md"), []byte("dirty two"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := adapter.fingerprint(context.Background(), " M", "cloud")
	if before == after {
		t.Fatalf("nested git worktree dirty content mutation did not change fingerprint: %q", before)
	}
}
