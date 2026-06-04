package git

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"errors"
	"fmt"
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
		".scafld/specs/archive/task.md":  []byte("# task\n"),
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
	for _, kept := range []string{".scafld/config.yaml", "api/handler.go"} {
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

func TestSnapshotStableTreeSHALeavesIndexAndHeadUntouched(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "file.txt", "one\n")
	head := commitSnapshotAll(t, root, "init")
	beforeIndex := snapshotIndexMD5(t, root)

	adapter := Adapter{Root: root}
	first, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if first.TreeSHA == "" || first.TreeSHA != second.TreeSHA {
		t.Fatalf("unstable tree sha: first=%q second=%q", first.TreeSHA, second.TreeSHA)
	}
	if afterIndex := snapshotIndexMD5(t, root); beforeIndex != afterIndex {
		t.Fatalf("real index changed: before=%s after=%s", beforeIndex, afterIndex)
	}
	if afterHead := gitSnapshot(t, root, "rev-parse", "HEAD"); strings.TrimSpace(afterHead) != head {
		t.Fatalf("HEAD changed: before=%s after=%s", head, afterHead)
	}
}

func TestSnapshotRejectsEmptyScope(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "file.txt", "one\n")
	commitSnapshotAll(t, root, "init")

	_, err := (Adapter{Root: root}).Snapshot(context.Background(), SnapshotInput{})
	if err == nil || !strings.Contains(err.Error(), "scope is empty") {
		t.Fatalf("Snapshot empty scope error = %v, want fail-closed scope error", err)
	}
}

func TestTreeDigestsNormalizesDirectoryScope(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "test/e2e/headline_path_test.go", "package e2e\n")
	commitSnapshotAll(t, root, "init")

	adapter := Adapter{Root: root}
	snap, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"test/e2e/"}})
	if err != nil {
		t.Fatal(err)
	}
	digests, err := adapter.TreeDigests(context.Background(), snap.TreeSHA, []string{"test/e2e/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(digests) != 1 || digests[0].Path != "test/e2e/headline_path_test.go" {
		t.Fatalf("TreeDigests with trailing-slash scope = %+v", digests)
	}
}

func TestSnapshotBaseHeadCommitSemantics(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "file.txt", "one\n")
	base := commitSnapshotAll(t, root, "base")
	writeSnapshotFile(t, root, "file.txt", "two\n")
	head := commitSnapshotAll(t, root, "head")

	adapter := Adapter{Root: root}
	withoutBase, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if withoutBase.HeadCommit != head || withoutBase.BaseCommit != head {
		t.Fatalf("without BaseRef: base=%s head=%s want head=%s", withoutBase.BaseCommit, withoutBase.HeadCommit, head)
	}
	withBase, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}, BaseRef: base})
	if err != nil {
		t.Fatal(err)
	}
	if withBase.HeadCommit != head || withBase.BaseCommit != base {
		t.Fatalf("with BaseRef: base=%s head=%s want base=%s head=%s", withBase.BaseCommit, withBase.HeadCommit, base, head)
	}
}

func TestIsAncestorWrapsGitMergeBase(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "file.txt", "base\n")
	base := commitSnapshotAll(t, root, "base")
	writeSnapshotFile(t, root, "file.txt", "head\n")
	head := commitSnapshotAll(t, root, "head")

	adapter := Adapter{Root: root}
	ok, err := adapter.IsAncestor(context.Background(), base, head)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("%s should be ancestor of %s", base, head)
	}
	ok, err = adapter.IsAncestor(context.Background(), head, base)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("%s should not be ancestor of %s", head, base)
	}
}

func TestSnapshotAutoCRLFAttributesParity(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, ".gitattributes", "*.txt text eol=lf\n")
	writeSnapshotFile(t, root, "line.txt", "one\r\ntwo\r\n")
	commitSnapshotAll(t, root, "attributes")

	adapter := Adapter{Root: root}
	gitSnapshot(t, root, "config", "core.autocrlf", "true")
	trueConfig, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	gitSnapshot(t, root, "config", "core.autocrlf", "false")
	falseConfig, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if trueConfig.TreeSHA != falseConfig.TreeSHA {
		t.Fatalf("core.autocrlf affected tree sha: true=%s false=%s", trueConfig.TreeSHA, falseConfig.TreeSHA)
	}
	digest, ok := snapshotDigestByPath(falseConfig, "line.txt")
	if !ok {
		t.Fatalf("line.txt missing from digests: %+v", falseConfig.FileDigests)
	}
	normalized := sha256.Sum256([]byte("one\ntwo\n"))
	if digest.SHA256 != fmt.Sprintf("%x", normalized) {
		t.Fatalf(".gitattributes normalization not reflected in blob digest: got %s", digest.SHA256)
	}
}

func TestSnapshotReadTreeTrackedIgnoredDeletion(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, ".gitignore", "*.log\n")
	writeSnapshotFile(t, root, "keep.log", "tracked ignored\n")
	gitSnapshot(t, root, "add", ".gitignore")
	gitSnapshot(t, root, "add", "-f", "keep.log")
	commitSnapshotAll(t, root, "tracked ignored")

	adapter := Adapter{Root: root}
	snapshot, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if digest, ok := snapshotDigestByPath(snapshot, "keep.log"); !ok || digest.Status != "unchanged" {
		t.Fatalf("tracked ignored file missing or wrong status: digest=%+v ok=%v", digest, ok)
	}
	if err := os.Remove(filepath.Join(root, "keep.log")); err != nil {
		t.Fatal(err)
	}
	deleted, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if !snapshotDeletedPath(deleted, "keep.log") {
		t.Fatalf("tracked ignored deletion missing: %+v", deleted.DeletedPaths)
	}
}

func TestSnapshotScafldRunsExcludedFromTree(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "app.go", "package app\n")
	writeSnapshotFile(t, root, ".scafld/runs/task/session.json", "{}\n")
	commitSnapshotAll(t, root, "runtime tracked")

	adapter := Adapter{Root: root}
	first, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	writeSnapshotFile(t, root, ".scafld/runs/task/session.json", "{\"changed\":true}\n")
	second, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if first.TreeSHA != second.TreeSHA {
		t.Fatalf(".scafld/runs changed tree sha: first=%s second=%s", first.TreeSHA, second.TreeSHA)
	}
	tree := gitSnapshot(t, root, "ls-tree", "-r", "--name-only", first.TreeSHA)
	if strings.Contains(tree, ".scafld/runs") {
		t.Fatalf("runtime path present in snapshot tree:\n%s", tree)
	}
}

func TestSnapshotDigestStatusDeletedIgnored(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, ".gitignore", "*.tmp\n")
	writeSnapshotFile(t, root, "unchanged.txt", "same\n")
	writeSnapshotFile(t, root, "modified.txt", "old\n")
	writeSnapshotFile(t, root, "deleted.txt", "bye\n")
	commitSnapshotAll(t, root, "base")
	writeSnapshotFile(t, root, "modified.txt", "new\n")
	writeSnapshotFile(t, root, "added.txt", "added\n")
	writeSnapshotFile(t, root, "ignored.tmp", "ignored\n")
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}

	snapshot, err := (Adapter{Root: root}).Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	for path, status := range map[string]string{
		"unchanged.txt": "unchanged",
		"modified.txt":  "modified",
		"added.txt":     "added",
	} {
		digest, ok := snapshotDigestByPath(snapshot, path)
		if !ok {
			t.Fatalf("%s missing from digests: %+v", path, snapshot.FileDigests)
		}
		if digest.Status != status {
			t.Fatalf("%s status=%s want %s", path, digest.Status, status)
		}
	}
	modifiedDigest, _ := snapshotDigestByPath(snapshot, "modified.txt")
	modifiedSum := sha256.Sum256([]byte("new\n"))
	if modifiedDigest.SHA256 != fmt.Sprintf("%x", modifiedSum) {
		t.Fatalf("modified digest=%s want %x", modifiedDigest.SHA256, modifiedSum)
	}
	if !snapshotDeletedPath(snapshot, "deleted.txt") {
		t.Fatalf("deleted path missing: %+v", snapshot.DeletedPaths)
	}
	if _, ok := snapshotDigestByPath(snapshot, "ignored.tmp"); ok {
		t.Fatalf("ignored file leaked into digests: %+v", snapshot.FileDigests)
	}
	if !snapshotIgnoredPath(snapshot, "ignored.tmp") {
		t.Fatalf("ignored file missing from ignored_unreviewed: %+v", snapshot.IgnoredUnreviewed)
	}
}

func TestSnapshotScopeFiltersDigestsIgnoredButTreeSHAIsFullRepo(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, ".gitignore", "src/*.tmp\noutside/*.tmp\n")
	writeSnapshotFile(t, root, "src/a.txt", "a\n")
	writeSnapshotFile(t, root, "outside/b.txt", "b\n")
	commitSnapshotAll(t, root, "base")

	adapter := Adapter{Root: root}
	before, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"src"}})
	if err != nil {
		t.Fatal(err)
	}
	writeSnapshotFile(t, root, "outside/b.txt", "changed outside\n")
	writeSnapshotFile(t, root, "outside/ignored.tmp", "ignored outside\n")
	writeSnapshotFile(t, root, "src/ignored.tmp", "ignored inside\n")
	after, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"src"}})
	if err != nil {
		t.Fatal(err)
	}
	if before.TreeSHA == after.TreeSHA {
		t.Fatal("outside-scope tracked content did not affect full-repository tree_sha")
	}
	for _, digest := range after.FileDigests {
		if !strings.HasPrefix(digest.Path, "src/") {
			t.Fatalf("outside-scope digest leaked: %+v", after.FileDigests)
		}
	}
	if !snapshotIgnoredPath(after, "src/ignored.tmp") {
		t.Fatalf("in-scope ignored path missing: %+v", after.IgnoredUnreviewed)
	}
	if snapshotIgnoredPath(after, "outside/ignored.tmp") {
		t.Fatalf("outside-scope ignored path leaked: %+v", after.IgnoredUnreviewed)
	}
}

func TestSnapshotIndexFlagEvidenceIntegrity(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "tracked.txt", "base\n")
	commitSnapshotAll(t, root, "base")
	adapter := Adapter{Root: root}

	gitSnapshot(t, root, "update-index", "--skip-worktree", "tracked.txt")
	_, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	var integrity EvidenceIntegrityError
	if !errors.As(err, &integrity) || !snapshotStringIn(integrity.Paths, "tracked.txt") {
		t.Fatalf("skip-worktree error=%v integrity=%+v", err, integrity)
	}
	gitSnapshot(t, root, "update-index", "--no-skip-worktree", "tracked.txt")

	gitSnapshot(t, root, "update-index", "--assume-unchanged", "tracked.txt")
	_, err = adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	integrity = EvidenceIntegrityError{}
	if !errors.As(err, &integrity) || !snapshotStringIn(integrity.Paths, "tracked.txt") {
		t.Fatalf("assume-unchanged error=%v integrity=%+v", err, integrity)
	}
	gitSnapshot(t, root, "update-index", "--no-assume-unchanged", "tracked.txt")

	// A skip-worktree marker on a non-ASCII filename must still fail closed.
	// Default git quoting renders the path as C-escaped bytes that miss the scope
	// match and silently pass; -z keeps the path raw so it is caught.
	writeSnapshotFile(t, root, "café.txt", "base\n")
	commitSnapshotAll(t, root, "unicode")
	gitSnapshot(t, root, "update-index", "--skip-worktree", "café.txt")
	_, err = adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	integrity = EvidenceIntegrityError{}
	if !errors.As(err, &integrity) || len(integrity.Paths) == 0 {
		t.Fatalf("unicode skip-worktree must fail closed: error=%v", err)
	}
	gitSnapshot(t, root, "update-index", "--no-skip-worktree", "café.txt")

	if _, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}}); err != nil {
		t.Fatalf("unflagged snapshot failed: %v", err)
	}
}

func TestSnapshotExcludesReceiptOutputFromTreeFingerprint(t *testing.T) {
	t.Parallel()

	root := initSnapshotRepo(t)
	writeSnapshotFile(t, root, "file.txt", "one\n")
	commitSnapshotAll(t, root, "init")

	adapter := Adapter{Root: root}
	before, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	// A passing finalize writes the signed receipt and projects the completed spec
	// after computing tree_sha. Those writes must not change the tree a later verify
	// recomputes, or an honest receipt would fail with a tree mismatch in the same
	// checkout.
	writeSnapshotFile(t, root, ".scafld/receipts/demo.json", `{"signed":true}`)
	writeSnapshotFile(t, root, ".scafld/specs/archive/demo.md", `# demo`)
	after, err := adapter.Snapshot(context.Background(), SnapshotInput{Scope: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if before.TreeSHA == "" || before.TreeSHA != after.TreeSHA {
		t.Fatalf("receipt write changed the tree fingerprint: before=%q after=%q", before.TreeSHA, after.TreeSHA)
	}
	for _, d := range after.FileDigests {
		if strings.HasPrefix(d.Path, ".scafld/receipts") || strings.HasPrefix(d.Path, ".scafld/specs") {
			t.Fatalf("scafld state file must never appear as reviewed evidence: %+v", d)
		}
	}
}

func TestIgnoredRuntimePathCoversScafldOutputs(t *testing.T) {
	t.Parallel()

	for _, p := range []string{".scafld/specs/demo.md", ".scafld/runs/x/session.json", ".scafld/receipts/demo.json", ".scafld/reviews/y.json"} {
		if !ignoredRuntimePath(p) {
			t.Fatalf("scafld runtime output must be excluded from review: %s", p)
		}
	}
	for _, p := range []string{"src/main.go", ".scafld/receiptsX/y"} {
		if ignoredRuntimePath(p) {
			t.Fatalf("reviewed work file must not be excluded: %s", p)
		}
	}
}

func initSnapshotRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", root).CombinedOutput(); err != nil {
		t.Skipf("git init unavailable: %v\n%s", err, out)
	}
	gitSnapshot(t, root, "config", "user.name", "scafld")
	gitSnapshot(t, root, "config", "user.email", "scafld@example.invalid")
	return root
}

func gitSnapshot(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeSnapshotFile(t *testing.T, root string, rel string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitSnapshotAll(t *testing.T, root string, message string) string {
	t.Helper()
	gitSnapshot(t, root, "add", "-A")
	gitSnapshot(t, root, "commit", "-m", message)
	return gitSnapshot(t, root, "rev-parse", "HEAD")
}

func snapshotIndexMD5(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".git", "index"))
	if err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum(data)
	return fmt.Sprintf("%x", sum)
}

func snapshotDigestByPath(snapshot Snapshot, path string) (FileDigest, bool) {
	for _, digest := range snapshot.FileDigests {
		if digest.Path == path {
			return digest, true
		}
	}
	return FileDigest{}, false
}

func snapshotDeletedPath(snapshot Snapshot, path string) bool {
	for _, deleted := range snapshot.DeletedPaths {
		if deleted.Path == path && deleted.Status == "deleted" {
			return true
		}
	}
	return false
}

func snapshotIgnoredPath(snapshot Snapshot, path string) bool {
	for _, ignored := range snapshot.IgnoredUnreviewed {
		if ignored.Path == path && ignored.Reason != "" {
			return true
		}
	}
	return false
}

func snapshotStringIn(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
