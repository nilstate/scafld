// Package specsource loads exact Markdown source for agent-facing commands.
package specsource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// ErrUnavailable is returned when a spec store cannot provide source Markdown.
var ErrUnavailable = errors.New("spec source markdown unavailable")

// ErrChanged is returned when the canonical Markdown changes while a provider
// packet is being assembled. The caller must discard the packet and start from
// a new source snapshot.
var ErrChanged = errors.New("spec source changed during operation")

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

// MarkdownDigest returns the digest of the exact canonical Markdown bytes.
// This is deliberately separate from spec.ContractDigest: lifecycle evidence
// can be projected from the ledger without changing the source contract, but a
// provider packet must never silently use an older source snapshot.
func MarkdownDigest(source spec.Source) string {
	sum := sha256.Sum256(source.Markdown)
	return hex.EncodeToString(sum[:])
}

// ReloadUnchanged reloads the canonical source and verifies that its path and
// exact bytes still match the snapshot used to assemble an agent packet.
func ReloadUnchanged(ctx context.Context, store any, prior spec.Source) (spec.Source, error) {
	current, err := Load(ctx, store, prior.Model.TaskID)
	if err != nil {
		return spec.Source{}, err
	}
	if strings.TrimSpace(current.Path) != strings.TrimSpace(prior.Path) || MarkdownDigest(current) != MarkdownDigest(prior) {
		return spec.Source{}, fmt.Errorf("%w: %s", ErrChanged, prior.Path)
	}
	return current, nil
}
