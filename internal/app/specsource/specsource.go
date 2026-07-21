// Package specsource loads exact Markdown source for agent-facing commands.
package specsource

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrUnavailable is returned when a spec store cannot provide source Markdown.
var ErrUnavailable = errors.New("spec source markdown unavailable")

// Loader is implemented by stores that can return the parsed spec and its
// exact Markdown source in one operation.
type Loader interface {
	LoadSource(context.Context, string) (spec.Source, error)
}

// Load returns the exact Markdown source for taskID.
func Load(ctx context.Context, store any, taskID string) (spec.Source, error) {
	loader, ok := store.(Loader)
	if !ok {
		return spec.Source{}, fmt.Errorf("%w: store does not implement LoadSource", ErrUnavailable)
	}
	source, err := loader.LoadSource(ctx, taskID)
	if err != nil {
		return spec.Source{}, err
	}
	if strings.TrimSpace(source.Path) == "" {
		return spec.Source{}, fmt.Errorf("%w: missing source path", ErrUnavailable)
	}
	if len(source.Markdown) == 0 {
		return spec.Source{}, fmt.Errorf("%w: missing source bytes for %s", ErrUnavailable, source.Path)
	}
	return source, nil
}
