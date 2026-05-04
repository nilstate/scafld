package prompts

import (
	"os"
	"path/filepath"

	coreprompts "github.com/nilstate/scafld/v2/internal/core/prompts"
)

// LoadHarden returns the project, core, or built-in hardening prompt.
func LoadHarden(root string) string {
	for _, rel := range []string{".scafld/prompts/harden.md", ".scafld/core/prompts/harden.md"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil {
			return string(data)
		}
	}
	return coreprompts.Harden
}
