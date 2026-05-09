package markdown

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/nilstate/scafld/v2/internal/core/spec"
	"github.com/nilstate/scafld/v2/internal/platform/atomicfile"
)

var (
	// ErrSpecNotFound is returned when taskID cannot be found in spec directories.
	ErrSpecNotFound = errors.New("spec not found")
	// ErrSpecExists is returned when a write would overwrite another task spec.
	ErrSpecExists = errors.New("spec already exists")
	// ErrMalformedMarkdown wraps spec Markdown grammar failures.
	ErrMalformedMarkdown = errors.New("malformed markdown spec")
)

// Store reads and writes Markdown task specs under a workspace root.
type Store struct {
	Root string
}

func (s Store) root() string {
	if s.Root == "" {
		return "."
	}
	return s.Root
}

// CreateDraft writes model as a new draft spec and returns its path.
func (s Store) CreateDraft(ctx context.Context, model spec.Model) (string, error) {
	path := filepath.Join(s.root(), ".scafld", "specs", "drafts", model.TaskID+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%w: %s", ErrSpecExists, path)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat draft spec: %w", err)
	}
	return path, writeSpec(ctx, path, model)
}

// Save writes model to an existing spec path.
func (s Store) Save(ctx context.Context, path string, model spec.Model) error {
	return updateSpec(ctx, s.root(), path, model)
}

// Load finds, parses, and returns a spec by task ID. If the file's directory
// disagrees with its frontmatter status, Load relocates it to the status-implied
// directory before returning. Relocation failures are non-fatal.
func (s Store) Load(ctx context.Context, taskID string) (spec.Model, string, error) {
	if err := ctx.Err(); err != nil {
		return spec.Model{}, "", err
	}
	path, err := s.Find(taskID)
	if err != nil {
		return spec.Model{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return spec.Model{}, "", fmt.Errorf("read spec %s: %w", path, err)
	}
	model, err := Parse(data)
	if err != nil {
		return spec.Model{}, "", err
	}
	if target := targetPath(s.root(), model); !samePath(path, target) {
		if err := writeMovedSpec(path, target, data); err == nil {
			path = target
		}
	}
	return model, path, nil
}

// Find returns the path for taskID across active spec directories.
func (s Store) Find(taskID string) (string, error) {
	root := s.root()
	for _, dir := range []string{"drafts", "approved", "active", "archive"} {
		base := filepath.Join(root, ".scafld", "specs", dir)
		if dir == "archive" {
			var found string
			err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() || filepath.Base(path) != taskID+".md" {
					return nil
				}
				found = path
				return filepath.SkipAll
			})
			if err == nil && found != "" {
				return found, nil
			}
			continue
		}
		path := filepath.Join(base, taskID+".md")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrSpecNotFound, taskID)
}

// List returns summaries for current non-archived task specs.
func (s Store) List(ctx context.Context) ([]spec.Record, error) {
	return s.list(ctx, false)
}

// ListAll returns summaries for current and archived task specs.
func (s Store) ListAll(ctx context.Context) ([]spec.Record, error) {
	return s.list(ctx, true)
}

func (s Store) list(ctx context.Context, includeArchive bool) ([]spec.Record, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := s.root()
	var records []spec.Record
	for _, dir := range []string{"drafts", "approved", "active"} {
		pattern := filepath.Join(root, ".scafld", "specs", dir, "*.md")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}
		for _, path := range matches {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			model, err := Parse(data)
			if err != nil {
				return nil, err
			}
			records = append(records, spec.Record{TaskID: model.TaskID, Status: model.Status, Path: path, Title: model.Title})
		}
	}
	if includeArchive {
		base := filepath.Join(root, ".scafld", "specs", "archive")
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			model, err := Parse(data)
			if err != nil {
				return err
			}
			records = append(records, spec.Record{TaskID: model.TaskID, Status: model.Status, Path: path, Title: model.Title})
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].TaskID < records[j].TaskID })
	return records, nil
}

func writeSpec(ctx context.Context, path string, model spec.Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create spec dir: %w", err)
	}
	data := Render(model)
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}

func updateSpec(ctx context.Context, root string, path string, model spec.Model) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read existing spec: %w", err)
	}
	previous, err := Parse(current)
	if err != nil {
		return err
	}
	if !samePhaseShape(previous.Phases, model.Phases) {
		return fmt.Errorf("%w: phase shape changed during targeted save", ErrMalformedMarkdown)
	}
	data, err := updateSpecMarkdown(current, previous, model)
	if err != nil {
		return err
	}
	target := targetPath(root, model)
	if samePath(path, target) {
		if err := atomicfile.Write(path, data, 0o644); err != nil {
			return fmt.Errorf("write spec: %w", err)
		}
		return nil
	}
	if err := writeMovedSpec(path, target, data); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}
	return nil
}

func targetPath(root string, model spec.Model) string {
	dir := "active"
	switch model.Status {
	case spec.StatusDraft:
		dir = "drafts"
	case spec.StatusApproved:
		dir = "approved"
	case spec.StatusCompleted, spec.StatusFailed, spec.StatusCancelled:
		return filepath.Join(root, ".scafld", "specs", "archive", archiveMonth(model.Updated), model.TaskID+".md")
	}
	return filepath.Join(root, ".scafld", "specs", dir, model.TaskID+".md")
}

func archiveMonth(updated string) string {
	if ts, err := time.Parse(time.RFC3339, updated); err == nil {
		return ts.UTC().Format("2006-01")
	}
	if len(updated) >= len("2006-01") {
		return updated[:len("2006-01")]
	}
	return time.Now().UTC().Format("2006-01")
}

func writeMovedSpec(source string, target string, data []byte) error {
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", ErrSpecExists, target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat target: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}
	if err := atomicfile.Write(target, data, 0o644); err != nil {
		return err
	}
	if err := os.Remove(source); err != nil {
		_ = os.Remove(target)
		return fmt.Errorf("remove old spec: %w", err)
	}
	return nil
}

func samePath(a string, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
