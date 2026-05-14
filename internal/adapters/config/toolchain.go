package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExecutionDetection records repo-declared toolchain hints that can make
// non-login acceptance shells behave like the checked-in project expects.
type ExecutionDetection struct {
	Execution ExecutionConfig
	Sources   []string
}

// DetectExecution infers safe PATH additions from checked-in toolchain files.
//
// Detection is intentionally limited to version-manager shim directories. It
// does not run shell startup files or infer per-command state. Project config
// remains the place for explicit environment variables and policy.
func DetectExecution(root string) ExecutionDetection {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	var paths []string
	var sources []string
	paths, sources = appendVersionFileShims(abs, paths, sources)
	paths, sources = appendVersionManagerShims(abs, paths, sources)
	for _, rel := range miseConfigMatches(abs) {
		if strings.TrimSpace(readToolchainText(abs, rel)) != "" {
			paths = append(paths, "$HOME/.local/share/mise/shims", "$HOME/.mise/shims")
			sources = append(sources, rel)
		}
	}
	return ExecutionDetection{
		Execution: ExecutionConfig{PathPrepend: dedupeOrdered(paths)},
		Sources:   dedupeOrdered(sources),
	}
}

// EffectiveExecution merges auto-detected toolchain shims behind explicit
// config. Explicit config paths appear first in PATH and explicit environment
// keys override detected values.
func EffectiveExecution(root string, explicit ExecutionConfig) ExecutionConfig {
	detected := DetectExecution(root).Execution
	return ExecutionConfig{
		PathPrepend:            dedupeOrdered(append(append([]string(nil), explicit.PathPrepend...), detected.PathPrepend...)),
		Env:                    overlayStrings(detected.Env, explicit.Env),
		AbsoluteTimeoutSeconds: explicit.AbsoluteTimeoutSeconds,
		IdleTimeoutSeconds:     explicit.IdleTimeoutSeconds,
	}
}

func appendVersionFileShims(root string, paths []string, sources []string) ([]string, []string) {
	for _, item := range []struct {
		file  string
		shims []string
	}{
		{".ruby-version", []string{"$HOME/.rbenv/shims"}},
		{".python-version", []string{"$HOME/.pyenv/shims"}},
		{".node-version", []string{"$HOME/.nodenv/shims"}},
		{".nvmrc", []string{"$HOME/.nodenv/shims"}},
		{".go-version", []string{"$HOME/.goenv/shims"}},
		{".java-version", []string{"$HOME/.jenv/shims"}},
	} {
		for _, rel := range matchingVersionFiles(root, item.file) {
			paths = append(paths, item.shims...)
			sources = append(sources, rel)
		}
	}
	return paths, sources
}

func appendVersionManagerShims(root string, paths []string, sources []string) ([]string, []string) {
	for _, rel := range matchingVersionFiles(root, ".tool-versions") {
		if strings.TrimSpace(readToolchainText(root, rel)) == "" {
			continue
		}
		paths = append(paths, "$HOME/.asdf/shims", "$HOME/.local/share/mise/shims", "$HOME/.mise/shims")
		sources = append(sources, rel)
	}
	return paths, sources
}

func matchingVersionFiles(root string, name string) []string {
	var out []string
	if hasFile(root, name) {
		out = append(out, name)
	}
	out = append(out, oneLevelFileMatches(root, name)...)
	return out
}

func readToolchainText(root string, rel string) string {
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(data)
}

func hasFile(root string, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func oneLevelFileMatches(root string, name string) []string {
	matches, err := filepath.Glob(filepath.Join(root, "*", name))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err == nil {
			out = append(out, filepath.ToSlash(rel))
		}
	}
	sort.Strings(out)
	return out
}

func miseConfigMatches(root string) []string {
	var out []string
	for _, rel := range []string{"mise.toml", ".mise.toml"} {
		if hasFile(root, rel) {
			out = append(out, rel)
		}
	}
	for _, name := range []string{"mise.toml", ".mise.toml"} {
		out = append(out, oneLevelFileMatches(root, name)...)
	}
	return dedupeOrdered(out)
}

func dedupeOrdered(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}
