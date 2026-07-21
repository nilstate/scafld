// Package agentcontract resolves source-backed role contracts for CLI commands.
package agentcontract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	corecontract "github.com/nilstate/scafld/v2/internal/core/agentcontract"
)

// Load resolves the workspace override, managed core copy, or embedded core
// asset for role.
func Load(ctx context.Context, root string, role corecontract.Role) (corecontract.Contract, error) {
	if err := ctx.Err(); err != nil {
		return corecontract.Contract{}, err
	}
	if !role.Valid() {
		return corecontract.Contract{}, fmt.Errorf("unknown agent contract role %q", role)
	}
	filename := role.Filename()
	for _, rel := range []string{
		filepath.ToSlash(filepath.Join(".scafld", "prompts", filename)),
		filepath.ToSlash(filepath.Join(".scafld", "core", "prompts", filename)),
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil {
			return corecontract.New(role, rel, data)
		}
		if !os.IsNotExist(err) {
			return corecontract.Contract{}, fmt.Errorf("read %s: %w", rel, err)
		}
	}
	data, err := corebundle.CorePrompt(filename)
	if err != nil {
		return corecontract.Contract{}, err
	}
	return corecontract.New(role, "embedded:.scafld/core/prompts/"+filename, data)
}
