package runartifact

import (
	"path/filepath"
	"strings"
)

const (
	scafldDir = ".scafld"
	runsDir   = "runs"
)

// RunDir returns the durable runtime directory for one task.
func RunDir(root string, taskID string) string {
	return filepath.Join(rootOrDot(root), scafldDir, runsDir, strings.TrimSpace(taskID))
}

// SessionPath returns the durable session ledger path for one task.
func SessionPath(root string, taskID string) string {
	return filepath.Join(RunDir(root, taskID), "session.json")
}

// CommandDiagnosticsDir returns the ignored command-output evidence directory
// for one task. The session ledger stores paths to relevant diagnostics.
func CommandDiagnosticsDir(root string, taskID string) string {
	return filepath.Join(RunDir(root, taskID), "artifacts", "commands")
}

// ProviderPacketsDir returns the ignored provider packet repair directory for
// one task. These artifacts are operator-editable recovery inputs, not ledgers.
func ProviderPacketsDir(root string, taskID string) string {
	return filepath.Join(RunDir(root, taskID), "artifacts", "provider-packets")
}

// SessionLockPath returns the ignored cross-process lock file path for one
// task's session ledger.
func SessionLockPath(root string, taskID string) string {
	return filepath.Join(rootOrDot(root), scafldDir, "locks", runsDir, SafeName(taskID)+".lock")
}

// SessionLockPathFromSessionPath returns the clean lock path for a conventional
// .scafld/runs/{task}/session.json path. Unknown layouts fall back to the legacy
// adjacent lock path so nonstandard callers still work.
func SessionLockPathFromSessionPath(path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "" || filepath.Base(clean) != "session.json" {
		return clean + ".lock"
	}
	taskDir := filepath.Dir(clean)
	taskID := filepath.Base(taskDir)
	runs := filepath.Dir(taskDir)
	scafld := filepath.Dir(runs)
	if filepath.Base(runs) != runsDir || filepath.Base(scafld) != scafldDir || strings.TrimSpace(taskID) == "" {
		return clean + ".lock"
	}
	return filepath.Join(scafld, "locks", runsDir, SafeName(taskID)+".lock")
}

// SafeName maps an operator-facing identifier into a stable filename fragment.
func SafeName(name string) string {
	mapped := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, strings.TrimSpace(name))
	return strings.Trim(mapped, "-.")
}

func rootOrDot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return "."
	}
	return root
}
