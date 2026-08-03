package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// EvidenceRoots returns absolute roots under which workspace evidence may be
// read or cited. The primary scafld root is always first.
func EvidenceRoots(root string, additional []string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		rootAbs = filepath.Clean(root)
	}
	roots := []string{filepath.Clean(rootAbs)}
	seen := map[string]bool{roots[0]: true}
	for _, value := range additional {
		text := expandEvidenceRoot(value)
		if text == "" {
			continue
		}
		if !filepath.IsAbs(text) {
			text = filepath.Join(rootAbs, filepath.FromSlash(text))
		}
		abs, err := filepath.Abs(text)
		if err != nil {
			abs = filepath.Clean(text)
		}
		abs = filepath.Clean(abs)
		if !seen[abs] {
			seen[abs] = true
			roots = append(roots, abs)
		}
	}
	return roots
}

func expandEvidenceRoot(value string) string {
	text := strings.TrimSpace(os.ExpandEnv(value))
	if text == "" {
		return ""
	}
	if text == "~" || strings.HasPrefix(text, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if text == "~" {
				return home
			}
			return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(text, "~/")))
		}
	}
	return text
}

// InsideRoot reports whether path is equal to or under root.
func InsideRoot(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// InsideAnyRoot reports whether path is equal to or under any root.
func InsideAnyRoot(path string, roots []string) bool {
	for _, root := range roots {
		if InsideRoot(path, root) {
			return true
		}
	}
	return false
}
