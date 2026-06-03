package git

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// State is the fingerprinted set of Git-visible workspace changes.
type State struct {
	Changed []string
}

// Adapter reads Git state from a workspace root.
type Adapter struct {
	Root string
}

// SnapshotInput controls a commit-free Git tree snapshot.
type SnapshotInput struct {
	Scope   []string
	BaseRef string
}

// Snapshot is a deterministic Git-backed workspace fingerprint.
type Snapshot struct {
	TreeSHA           string
	BaseCommit        string
	HeadCommit        string
	FileDigests       []FileDigest
	DeletedPaths      []DeletedPath
	IgnoredUnreviewed []IgnoredPath
}

// FileDigest records a present path in the snapshot tree.
type FileDigest struct {
	Path   string
	Status string
	SHA256 string
}

// DeletedPath records an in-scope path deleted from the base commit.
type DeletedPath struct {
	Path   string
	Status string
}

// IgnoredPath records an ignored path that was intentionally excluded.
type IgnoredPath struct {
	Path   string
	Reason string
}

// EvidenceIntegrityError reports index flags that can hide working-tree edits.
type EvidenceIntegrityError struct {
	Paths []string
}

func (e EvidenceIntegrityError) Error() string {
	return "evidence_integrity: git index flags hide working-tree evidence: " + strings.Join(e.Paths, ", ")
}

// Status returns the current changed-file fingerprints.
func (a Adapter) Status(ctx context.Context) (State, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1", "--untracked-files=all")
	cmd.Dir = a.Root
	out, err := cmd.Output()
	if err != nil {
		return State{}, err
	}
	var changed []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > 3 {
			path := strings.TrimSpace(line[3:])
			if ignoredRuntimePath(path) {
				continue
			}
			changed = append(changed, a.fingerprint(ctx, line[:2], path))
		}
	}
	sort.Strings(changed)
	return State{Changed: changed}, nil
}

// scafldRuntimePaths are scafld-owned runtime output directories that are never
// part of the reviewed work tree. They are the single source of truth for both
// the per-file digests (ignoredRuntimePath) and the tree fingerprint
// (writeTemporaryTree), so a gate that writes a receipt or session ledger under
// .scafld cannot change the tree_sha it just signed.
var scafldRuntimePaths = []string{
	".scafld/runs",
	".scafld/reviews",
	".scafld/receipts",
}

func ignoredRuntimePath(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, prefix := range scafldRuntimePaths {
		if strings.HasPrefix(normalized+"/", prefix+"/") {
			return true
		}
	}
	return false
}

// ChangedFiles returns changed-file fingerprints for mutation guards.
func (a Adapter) ChangedFiles(ctx context.Context) ([]string, error) {
	state, err := a.Status(ctx)
	if err != nil {
		return nil, err
	}
	return state.Changed, nil
}

// Snapshot writes the working tree to a temporary Git index and returns a
// content-addressed fingerprint without mutating the real index, HEAD, or stash.
func (a Adapter) Snapshot(ctx context.Context, input SnapshotInput) (Snapshot, error) {
	scope := normalizeScope(input.Scope)
	if err := a.checkIndexFlags(ctx, scope); err != nil {
		return Snapshot{}, err
	}

	headCommit, hasHead, err := a.resolveHead(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	baseCommit := ""
	if hasHead {
		baseCommit = headCommit
		if strings.TrimSpace(input.BaseRef) != "" {
			out, err := a.gitOutput(ctx, nil, "merge-base", input.BaseRef, "HEAD")
			if err != nil {
				return Snapshot{}, err
			}
			baseCommit = strings.TrimSpace(string(out))
		}
	}

	treeSHA, err := a.writeTemporaryTree(ctx, hasHead)
	if err != nil {
		return Snapshot{}, err
	}
	statuses, deleted, err := a.diffStatuses(ctx, baseCommit, treeSHA, scope)
	if err != nil {
		return Snapshot{}, err
	}
	digests, err := a.fileDigests(ctx, treeSHA, statuses, scope)
	if err != nil {
		return Snapshot{}, err
	}
	ignored, err := a.ignoredPaths(ctx, scope)
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		TreeSHA:           treeSHA,
		BaseCommit:        baseCommit,
		HeadCommit:        headCommit,
		FileDigests:       digests,
		DeletedPaths:      deleted,
		IgnoredUnreviewed: ignored,
	}, nil
}

// IsAncestor reports whether ancestor is reachable from descendant.
func (a Adapter) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, err := a.gitOutput(ctx, nil, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, errGitCommand) {
		return false, nil
	}
	return false, err
}

func (a Adapter) fingerprint(ctx context.Context, status string, rel string) string {
	path := filepath.Join(a.Root, filepath.FromSlash(rel))
	return a.fingerprintPath(ctx, status, rel, path, true)
}

func (a Adapter) fingerprintPath(ctx context.Context, status string, rel string, path string, detectGitWorktree bool) string {
	info, err := os.Stat(path)
	if err != nil {
		return status + " deleted " + rel
	}
	if info.IsDir() {
		if detectGitWorktree {
			if fp, ok := a.gitWorktreeFingerprint(ctx, status, rel, path); ok {
				return fp
			}
		}
		sum, err := directoryHash(path)
		if err != nil {
			return status + " unreadable " + rel
		}
		return fmt.Sprintf("%s %s %s", status, sum, rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return status + " unreadable " + rel
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%s %x %s", status, sum, rel)
}

func (a Adapter) gitWorktreeFingerprint(ctx context.Context, status string, rel string, path string) (string, bool) {
	head := exec.CommandContext(ctx, "git", "-C", path, "rev-parse", "HEAD")
	headOut, err := head.Output()
	if err != nil {
		return "", false
	}
	state := exec.CommandContext(ctx, "git", "-C", path, "status", "--porcelain=v1")
	stateOut, err := state.Output()
	if err != nil {
		return "", false
	}
	h := sha256.New()
	if _, err := io.WriteString(h, "HEAD "+strings.TrimSpace(string(headOut))+"\n"); err != nil {
		return "", false
	}
	for _, fp := range a.gitWorktreeChangedFingerprints(ctx, rel, path, string(stateOut)) {
		if _, err := io.WriteString(h, fp+"\n"); err != nil {
			return "", false
		}
	}
	return fmt.Sprintf("%s %x %s", status, h.Sum(nil), rel), true
}

func (a Adapter) gitWorktreeChangedFingerprints(ctx context.Context, rel string, path string, statusText string) []string {
	var changed []string
	for _, line := range strings.Split(statusText, "\n") {
		if strings.TrimSpace(line) == "" || len(line) <= 3 {
			continue
		}
		status := line[:2]
		nestedRel := strings.TrimSpace(line[3:])
		if _, renamedTo, ok := strings.Cut(nestedRel, " -> "); ok {
			nestedRel = strings.TrimSpace(renamedTo)
		}
		displayRel := filepath.ToSlash(filepath.Join(rel, filepath.FromSlash(nestedRel)))
		abs := filepath.Join(path, filepath.FromSlash(nestedRel))
		changed = append(changed, a.fingerprintPath(ctx, status, displayRel, abs, false))
	}
	sort.Strings(changed)
	return changed
}

func (a Adapter) writeTemporaryTree(ctx context.Context, hasHead bool) (string, error) {
	dir, err := os.MkdirTemp("", "scafld-git-index-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	indexPath := filepath.Join(dir, "index")
	env := []string{"GIT_INDEX_FILE=" + indexPath}
	if hasHead {
		if _, err := a.gitOutput(ctx, env, "-c", "core.autocrlf=false", "read-tree", "HEAD"); err != nil {
			return "", err
		}
	} else if _, err := a.gitOutput(ctx, env, "-c", "core.autocrlf=false", "read-tree", "--empty"); err != nil {
		return "", err
	}
	if _, err := a.gitOutput(ctx, env, "-c", "core.autocrlf=false", "add", "--all"); err != nil {
		return "", err
	}
	rmArgs := append([]string{"-c", "core.autocrlf=false", "rm", "-r", "--cached", "--ignore-unmatch", "--"}, scafldRuntimePaths...)
	if _, err := a.gitOutput(ctx, env, rmArgs...); err != nil {
		return "", err
	}
	out, err := a.gitOutput(ctx, env, "-c", "core.autocrlf=false", "write-tree")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (a Adapter) resolveHead(ctx context.Context) (string, bool, error) {
	out, err := a.gitOutput(ctx, nil, "rev-parse", "--verify", "HEAD")
	if err != nil {
		if errors.Is(err, errGitCommand) {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(string(out)), true, nil
}

// TreeDigests lists the in-scope file digests of an existing tree object,
// reading the immutable tree rather than the working directory. The gate reviewer
// uses it to read the exact bytes the receipt signs, regardless of any later
// working-tree mutation.
func (a Adapter) TreeDigests(ctx context.Context, treeSHA string, scope []string) ([]FileDigest, error) {
	return a.fileDigests(ctx, treeSHA, map[string]string{}, scope)
}

// CanonicalBytes returns the committed blob bytes for path at the snapshot tree.
// It is the reviewer-evidence source: bytes come from the content-addressed tree
// object, never the mutable working file, so the reviewer reads exactly what the
// receipt fingerprints.
func (a Adapter) CanonicalBytes(ctx context.Context, treeSHA string, path string) ([]byte, error) {
	if strings.TrimSpace(treeSHA) == "" || strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("canonical bytes require a tree and path")
	}
	return a.gitOutput(ctx, nil, "cat-file", "blob", treeSHA+":"+path)
}

var errGitCommand = errors.New("git command failed")

func (a Adapter) gitOutput(ctx context.Context, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.Root
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%w: git %s: %v\n%s", errGitCommand, strings.Join(args, " "), err, out)
	}
	return out, nil
}

func (a Adapter) checkIndexFlags(ctx context.Context, scope []string) error {
	// git ls-files -v marks skip-worktree as 'S' and lowercases the tag whenever
	// assume-unchanged is set; both hide local edits from review. -z keeps paths
	// raw so a quoted unicode/special filename cannot slip past the scope match
	// and fail open.
	args := append([]string{"ls-files", "-v", "-z", "--"}, scope...)
	out, err := a.gitOutput(ctx, nil, args...)
	if err != nil {
		return err
	}
	var flagged []string
	for _, entry := range strings.Split(string(out), "\x00") {
		if len(entry) < 3 {
			continue
		}
		flag := entry[0]
		path := entry[2:]
		// Fail closed on skip-worktree (S) or any assume-unchanged (lowercase)
		// tag. Normal cached files report uppercase H and are left alone.
		if (flag == 'S' || (flag >= 'a' && flag <= 'z')) && pathInScope(path, scope) && !ignoredRuntimePath(path) {
			flagged = append(flagged, path)
		}
	}
	if len(flagged) > 0 {
		sort.Strings(flagged)
		return EvidenceIntegrityError{Paths: flagged}
	}
	return nil
}

func (a Adapter) diffStatuses(ctx context.Context, baseCommit string, treeSHA string, scope []string) (map[string]string, []DeletedPath, error) {
	statuses := map[string]string{}
	if baseCommit == "" {
		return statuses, nil, nil
	}
	// diff.renames=false keeps recorded statuses deterministic across hosts: a
	// move records the new path as added and the old as deleted instead of a
	// host-config-dependent rename, so coverage cannot drift with git settings.
	out, err := a.gitOutput(ctx, nil, "-c", "diff.renames=false", "diff", "--name-status", "-z", baseCommit, treeSHA)
	if err != nil {
		return nil, nil, err
	}
	fields := strings.Split(string(out), "\x00")
	var deleted []DeletedPath
	for i := 0; i < len(fields)-1; {
		status := fields[i]
		i++
		if status == "" || i >= len(fields) {
			continue
		}
		code := status[:1]
		if code == "R" || code == "C" {
			if i+1 >= len(fields) {
				break
			}
			i++
			newPath := fields[i]
			i++
			if pathInScope(newPath, scope) && !ignoredRuntimePath(newPath) {
				statuses[newPath] = "modified"
			}
			continue
		}
		path := fields[i]
		i++
		if !pathInScope(path, scope) || ignoredRuntimePath(path) {
			continue
		}
		switch code {
		case "A":
			statuses[path] = "added"
		case "D":
			deleted = append(deleted, DeletedPath{Path: path, Status: "deleted"})
		default:
			statuses[path] = "modified"
		}
	}
	sort.Slice(deleted, func(i, j int) bool {
		return deleted[i].Path < deleted[j].Path
	})
	return statuses, deleted, nil
}

func (a Adapter) fileDigests(ctx context.Context, treeSHA string, statuses map[string]string, scope []string) ([]FileDigest, error) {
	out, err := a.gitOutput(ctx, nil, "ls-tree", "-rz", "-r", "--full-tree", treeSHA)
	if err != nil {
		return nil, err
	}
	var digests []FileDigest
	for _, entry := range strings.Split(string(out), "\x00") {
		if entry == "" {
			continue
		}
		meta, path, ok := strings.Cut(entry, "\t")
		if !ok || !pathInScope(path, scope) || ignoredRuntimePath(path) {
			continue
		}
		parts := strings.Fields(meta)
		if len(parts) < 3 {
			continue
		}
		mode, objectType, oid := parts[0], parts[1], parts[2]
		status := statuses[path]
		if status == "" {
			status = "unchanged"
		}
		sum := ""
		if mode == "160000" || objectType == "commit" {
			status = "gitlink"
			h := sha256.Sum256([]byte("gitlink\x00" + oid))
			sum = fmt.Sprintf("%x", h)
		} else {
			data, err := a.gitOutput(ctx, nil, "cat-file", "blob", oid)
			if err != nil {
				return nil, err
			}
			h := sha256.Sum256(data)
			sum = fmt.Sprintf("%x", h)
		}
		digests = append(digests, FileDigest{Path: path, Status: status, SHA256: sum})
	}
	sort.Slice(digests, func(i, j int) bool {
		return digests[i].Path < digests[j].Path
	})
	return digests, nil
}

func (a Adapter) ignoredPaths(ctx context.Context, scope []string) ([]IgnoredPath, error) {
	// -z keeps ignored paths raw (no C-quoting) so special filenames are recorded
	// and scope-matched as their real bytes.
	args := []string{"status", "--porcelain=v1", "-z", "--ignored", "--untracked-files=all", "--"}
	args = append(args, scope...)
	out, err := a.gitOutput(ctx, nil, args...)
	if err != nil {
		return nil, err
	}
	seen := map[string]IgnoredPath{}
	for _, entry := range strings.Split(string(out), "\x00") {
		if !strings.HasPrefix(entry, "!! ") {
			continue
		}
		path := entry[3:]
		if path == "" || ignoredRuntimePath(path) || !pathInScope(path, scope) {
			continue
		}
		seen[path] = IgnoredPath{Path: path, Reason: a.ignoreReason(ctx, path)}
	}
	ignored := make([]IgnoredPath, 0, len(seen))
	for _, item := range seen {
		ignored = append(ignored, item)
	}
	sort.Slice(ignored, func(i, j int) bool {
		return ignored[i].Path < ignored[j].Path
	})
	return ignored, nil
}

func (a Adapter) ignoreReason(ctx context.Context, path string) string {
	out, err := a.gitOutput(ctx, nil, "check-ignore", "-v", "--", path)
	if err != nil {
		return "ignored"
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return "ignored"
	}
	if before, _, ok := strings.Cut(line, "\t"); ok {
		return before
	}
	return line
}

func normalizeScope(scope []string) []string {
	var normalized []string
	seen := map[string]bool{}
	for _, item := range scope {
		path := strings.Trim(strings.ReplaceAll(item, "\\", "/"), "/")
		path = strings.TrimPrefix(path, "./")
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		normalized = append(normalized, path)
	}
	sort.Strings(normalized)
	return normalized
}

func pathInScope(path string, scope []string) bool {
	if len(scope) == 0 {
		return true
	}
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, prefix := range scope {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
}

func directoryHash(path string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(path, func(item string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(path, item)
		if err != nil {
			return err
		}
		if _, err := io.WriteString(h, filepath.ToSlash(rel)+"\n"); err != nil {
			return err
		}
		data, err := os.ReadFile(item)
		if err != nil {
			return err
		}
		if _, err := h.Write(data); err != nil {
			return err
		}
		if _, err := io.WriteString(h, "\n"); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
