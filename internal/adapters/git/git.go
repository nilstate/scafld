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
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1")
	cmd.Dir = a.Root
	out, err := cmd.Output()
	if err != nil {
		return State{}, err
	}
	var changed []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) > 3 {
			path := strings.TrimSpace(line[3:])
			changed = append(changed, a.fingerprint(ctx, line[:2], path))
		}
	}
	sort.Strings(changed)
	return State{Changed: changed}, nil
}

// ChangedFiles returns changed-file fingerprints for mutation guards.
func (a Adapter) ChangedFiles(ctx context.Context) ([]string, error) {
	state, err := a.Status(ctx)
	if err != nil {
		return nil, nil
	}
	return state.Changed, nil
}

func (a Adapter) fingerprint(ctx context.Context, status string, rel string) string {
	path := filepath.Join(a.Root, filepath.FromSlash(rel))
	info, err := os.Stat(path)
	if err != nil {
		return status + " deleted " + rel
	}
	if info.IsDir() {
		if fp, ok := a.gitWorktreeFingerprint(ctx, status, rel, path); ok {
			return fp
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
	sum := sha256.Sum256(append([]byte(strings.TrimSpace(string(headOut))+"\n"), stateOut...))
	return fmt.Sprintf("%s %x %s", status, sum, rel), true
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
