package providers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/reviewevidence"
)

// SandboxPolicy describes the provider-visible limits applied to a
// receipt-grade review run.
type SandboxPolicy struct {
	ReadRoots []string `json:"read_roots"`
	// ReadRootsEnforced is true only when the provider CLI hard-confines the
	// reviewer's reads to ReadRoots (Claude --add-dir, Gemini includeDirectories).
	// For Codex it is false: its read-only sandbox plus working directory prevent
	// writes and default reads outside ReadRoots, but do not jail them, so the
	// receipt records best-effort confinement honestly rather than claiming a jail.
	ReadRootsEnforced       bool     `json:"read_roots_enforced"`
	MemoryAutoloadDisabled  bool     `json:"memory_autoload_disabled"`
	AgentInstructionBlocked []string `json:"agent_instruction_blocked,omitempty"`
}

// SandboxArgsPolicy records provider-specific evidence sandbox controls.
type SandboxArgsPolicy struct {
	ReadRoots              []string `json:"read_roots"`
	MemoryAutoloadDisabled bool     `json:"memory_autoload_disabled"`
	Provider               string   `json:"provider,omitempty"`
}

// EvidenceSandbox is the materialized scratch directory and its cleanup owner.
type EvidenceSandbox struct {
	CWD        string
	Home       string
	ReadRoots  []string
	Env        []string
	ArgsPolicy SandboxArgsPolicy
	Provenance []reviewevidence.Provenance
	Policy     SandboxPolicy
	Cleanup    func()
}

// BuildEvidenceSandbox verifies and materializes canonical file evidence into
// a scratch directory that is safe to expose to a reviewer subprocess.
func BuildEvidenceSandbox(files []reviewevidence.EvidenceFile) (EvidenceSandbox, error) {
	validated, err := validateEvidenceFiles(files)
	if err != nil {
		return EvidenceSandbox{}, err
	}
	root, err := os.MkdirTemp("", "scafld-review-evidence-")
	if err != nil {
		return EvidenceSandbox{}, err
	}
	// An empty home dir, separate from the evidence root, becomes the reviewer's
	// HOME/XDG_CONFIG_HOME so host user-global agent memory (~/.claude, ~/.gemini)
	// cannot autoload and launder context into the review.
	home, err := os.MkdirTemp("", "scafld-review-home-")
	if err != nil {
		_ = os.RemoveAll(root)
		return EvidenceSandbox{}, err
	}
	cleanup := func() {
		_ = os.RemoveAll(root)
		_ = os.RemoveAll(home)
	}
	sandbox := EvidenceSandbox{
		CWD:       root,
		Home:      home,
		ReadRoots: []string{root},
		ArgsPolicy: SandboxArgsPolicy{
			ReadRoots:              []string{root},
			MemoryAutoloadDisabled: true,
		},
		Policy: SandboxPolicy{
			ReadRoots:               []string{root},
			MemoryAutoloadDisabled:  true,
			AgentInstructionBlocked: []string{"CLAUDE.md", "AGENTS.md", ".scafld/config.yaml", "GEMINI.md"},
		},
		Cleanup: cleanup,
	}
	for _, file := range validated {
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if !pathInside(root, target) {
			cleanup()
			return EvidenceSandbox{}, fmt.Errorf("evidence path escapes sandbox: %s", file.Path)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			cleanup()
			return EvidenceSandbox{}, err
		}
		if err := os.WriteFile(target, file.Bytes, 0o600); err != nil {
			cleanup()
			return EvidenceSandbox{}, err
		}
		sandbox.Provenance = append(sandbox.Provenance, reviewevidence.Provenance{
			Path:        file.Path,
			Status:      file.Status,
			SHA256:      file.SHA256,
			ScratchPath: target,
		})
	}
	return sandbox, nil
}

func validateEvidenceFiles(files []reviewevidence.EvidenceFile) ([]reviewevidence.EvidenceFile, error) {
	validated := make([]reviewevidence.EvidenceFile, 0, len(files))
	seen := map[string]bool{}
	for _, file := range files {
		next, err := reviewevidence.ValidateFile(file)
		if err != nil {
			return nil, err
		}
		if err := rejectBlocklistedEvidencePath(next.Path); err != nil {
			return nil, err
		}
		if seen[next.Path] {
			return nil, fmt.Errorf("duplicate evidence path: %s", next.Path)
		}
		seen[next.Path] = true
		validated = append(validated, next)
	}
	return validated, nil
}

func rejectBlocklistedEvidencePath(path string) error {
	normalized, err := reviewevidence.NormalizePath(path)
	if err != nil {
		return err
	}
	base := filepath.Base(filepath.FromSlash(normalized))
	switch base {
	case "CLAUDE.md", "AGENTS.md", "GEMINI.md":
		return fmt.Errorf("agent instruction file cannot be pinned as evidence: %s", normalized)
	}
	if normalized == ".scafld/config.yaml" {
		return fmt.Errorf("scafld local config cannot be pinned as evidence: %s", normalized)
	}
	return nil
}

func pathInside(root string, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
