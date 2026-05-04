package git

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type State struct {
	Changed []string
}

type Adapter struct {
	Root string
}

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
			changed = append(changed, a.fingerprint(line[:2], path))
		}
	}
	sort.Strings(changed)
	return State{Changed: changed}, nil
}

func (a Adapter) ChangedFiles(ctx context.Context) ([]string, error) {
	state, err := a.Status(ctx)
	if err != nil {
		return nil, nil
	}
	return state.Changed, nil
}

func (a Adapter) fingerprint(status string, rel string) string {
	path := filepath.Join(a.Root, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		return status + " deleted " + rel
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%s %x %s", status, sum, rel)
}
