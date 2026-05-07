package git

import (
	"context"
	"crypto/sha256"
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

func ignoredRuntimePath(path string) bool {
	normalized := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	for _, prefix := range []string{
		".scafld/runs/",
	} {
		if strings.HasPrefix(normalized+"/", prefix) {
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
