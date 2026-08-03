// Package agentcontract resolves source-backed role contracts for CLI commands.
package agentcontract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/adapters/corebundle"
	corecontract "github.com/nilstate/scafld/v2/internal/core/agentcontract"
)

// Load resolves the source-backed role contract. Runtime authority is the
// embedded contract unless a workspace prompt explicitly opts into project
// ownership with agentcontract.ProjectOverrideMarker.
func Load(ctx context.Context, root string, role corecontract.Role) (corecontract.Contract, error) {
	if err := ctx.Err(); err != nil {
		return corecontract.Contract{}, err
	}
	if !role.Valid() {
		return corecontract.Contract{}, fmt.Errorf("unknown agent contract role %q", role)
	}
	filename := role.Filename()
	projectRel := filepath.ToSlash(filepath.Join(".scafld", "prompts", filename))
	if data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(projectRel))); err == nil {
		if corecontract.HasProjectOverrideMarker(data) {
			return corecontract.New(role, projectRel, data)
		}
	} else if !os.IsNotExist(err) {
		return corecontract.Contract{}, fmt.Errorf("read %s: %w", projectRel, err)
	}

	data, err := corebundle.CorePrompt(filename)
	if err == nil {
		return corecontract.New(role, "embedded:.scafld/core/prompts/"+filename, data)
	}

	coreRel := filepath.ToSlash(filepath.Join(".scafld", "core", "prompts", filename))
	if coreData, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(coreRel))); readErr == nil {
		if strings.TrimSpace(string(coreData)) != "" {
			return corecontract.New(role, coreRel, coreData)
		}
	} else if !os.IsNotExist(readErr) {
		return corecontract.Contract{}, fmt.Errorf("read %s: %w", coreRel, readErr)
	}
	return corecontract.Contract{}, err
}
