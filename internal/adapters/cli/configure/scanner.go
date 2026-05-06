package configure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	appconfigure "github.com/nilstate/scafld/v2/internal/app/configure"
)

// Scanner inspects a workspace for config-relevant evidence.
type Scanner struct {
	Root string
}

// Scan returns repo evidence without mutating the workspace.
func (s Scanner) Scan(ctx context.Context) (appconfigure.Snapshot, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return appconfigure.Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	snapshot := appconfigure.Snapshot{Root: abs}
	for _, candidate := range fileCandidates() {
		if err := ctx.Err(); err != nil {
			return appconfigure.Snapshot{}, err
		}
		if exists(filepath.Join(abs, filepath.FromSlash(candidate.Path))) {
			snapshot.Files = append(snapshot.Files, candidate)
		}
	}
	workflowFiles, err := globRel(abs, ".github/workflows/*")
	if err != nil {
		return appconfigure.Snapshot{}, err
	}
	for _, rel := range workflowFiles {
		if strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml") {
			snapshot.Files = append(snapshot.Files, appconfigure.Evidence{Path: rel, Role: "ci_workflow"})
		}
	}
	snapshot.Commands = commandSuggestions(abs)
	snapshot.Invariants = invariantSuggestions(abs, snapshot.Files)
	snapshot.Warnings = legacyConfigWarnings(abs)
	snapshot.Questions = openQuestions(snapshot)
	return snapshot, nil
}

func fileCandidates() []appconfigure.Evidence {
	return []appconfigure.Evidence{
		{Path: "AGENTS.md", Role: "agent_contract"},
		{Path: "CLAUDE.md", Role: "claude_agent_contract"},
		{Path: "README.md", Role: "project_overview"},
		{Path: "Makefile", Role: "command_surface"},
		{Path: "justfile", Role: "command_surface"},
		{Path: "Taskfile.yml", Role: "command_surface"},
		{Path: "Taskfile.yaml", Role: "command_surface"},
		{Path: "go.mod", Role: "go_module"},
		{Path: "package.json", Role: "node_package"},
		{Path: "pnpm-workspace.yaml", Role: "node_workspace"},
		{Path: "pyproject.toml", Role: "python_package"},
		{Path: "Cargo.toml", Role: "rust_package"},
		{Path: "Gemfile", Role: "ruby_package"},
		{Path: "internal/arch/architecture_test.go", Role: "architecture_gate"},
	}
}

func commandSuggestions(root string) []appconfigure.CommandSuggestion {
	var commands []appconfigure.CommandSuggestion
	if makefile := readText(filepath.Join(root, "Makefile")); makefile != "" {
		if hasMakeTarget(makefile, "check") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "full_check", Command: "make check", Sources: []string{"Makefile"}})
		}
		if hasMakeTarget(makefile, "test") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "test", Command: "make test", Sources: []string{"Makefile"}})
		}
	}
	if packageJSON := readText(filepath.Join(root, "package.json")); packageJSON != "" {
		var pkg struct {
			Scripts map[string]string `json:"scripts"`
		}
		if json.Unmarshal([]byte(packageJSON), &pkg) == nil {
			if _, ok := pkg.Scripts["test"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "test", Command: "npm test", Sources: []string{"package.json:scripts.test"}})
			}
			if _, ok := pkg.Scripts["lint"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "lint", Command: "npm run lint", Sources: []string{"package.json:scripts.lint"}})
			}
			if _, ok := pkg.Scripts["typecheck"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "typecheck", Command: "npm run typecheck", Sources: []string{"package.json:scripts.typecheck"}})
			}
		}
	}
	return dedupeCommands(commands)
}

func invariantSuggestions(root string, files []appconfigure.Evidence) []appconfigure.InvariantSuggestion {
	var invariants []appconfigure.InvariantSuggestion
	if evidenceRole(files, "architecture_gate") {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "architecture_boundaries",
			Description: "Preserve the architecture boundaries enforced by the repo's architecture tests.",
			Sources:     []string{"internal/arch/architecture_test.go"},
		})
	}
	if evidenceRole(files, "ci_workflow") {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "ci_must_pass",
			Description: "Changes should preserve the committed CI workflow expectations.",
			Sources:     pathsByRole(files, "ci_workflow"),
		})
	}
	if readText(filepath.Join(root, "go.mod")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "go_module_integrity",
			Description: "Keep the Go module buildable and avoid unchecked dependency or module-path drift.",
			Sources:     []string{"go.mod"},
		})
	}
	if readText(filepath.Join(root, "package.json")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "package_script_integrity",
			Description: "Keep package scripts and generated package metadata aligned with release behavior.",
			Sources:     []string{"package.json"},
		})
	}
	return dedupeInvariants(invariants)
}

func openQuestions(snapshot appconfigure.Snapshot) []appconfigure.Question {
	var questions []appconfigure.Question
	if !hasCommand(snapshot.Commands, "full_check") {
		questions = append(questions, appconfigure.Question{
			Question: "What single command is authoritative before release or commit?",
			Reason:   "No `make check` target was detected.",
		})
	}
	if len(snapshot.Invariants) == 0 {
		questions = append(questions, appconfigure.Question{
			Question: "Which project-specific invariants must every spec consider?",
			Reason:   "No architecture gate, CI workflow, or recognized package manifest implied a concrete invariant.",
		})
	}
	return questions
}

func legacyConfigWarnings(root string) []appconfigure.Warning {
	data, err := os.ReadFile(filepath.Join(root, ".scafld", "config.yaml"))
	if err != nil {
		return nil
	}
	text := string(data)
	var found []string
	for _, key := range []string{"modes", "validation", "rubric", "react", "tech_stack", "repo_layout", "communication", "safety"} {
		if regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `:`).MatchString(text) {
			found = append(found, key)
		}
	}
	if len(found) == 0 {
		return nil
	}
	sort.Strings(found)
	return []appconfigure.Warning{{
		ID:      "legacy_ignored_config_keys",
		Message: "The current .scafld/config.yaml contains keys the Go runtime does not read: " + strings.Join(found, ", ") + ". Move real policy into invariants, specs, acceptance criteria, or review passes before deleting stale keys.",
		Sources: []string{".scafld/config.yaml"},
	}}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readText(path string) string {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return ""
	}
	return string(data)
}

var makeTarget = regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):`)

func hasMakeTarget(text string, target string) bool {
	for _, match := range makeTarget.FindAllStringSubmatch(text, -1) {
		if len(match) == 2 && match[1] == target {
			return true
		}
	}
	return false
}

func globRel(root string, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err != nil {
			return nil, err
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out, nil
}

func evidenceRole(files []appconfigure.Evidence, role string) bool {
	return len(pathsByRole(files, role)) > 0
}

func pathsByRole(files []appconfigure.Evidence, role string) []string {
	var paths []string
	for _, file := range files {
		if file.Role == role {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func hasCommand(commands []appconfigure.CommandSuggestion, id string) bool {
	for _, command := range commands {
		if command.ID == id {
			return true
		}
	}
	return false
}

func dedupeCommands(commands []appconfigure.CommandSuggestion) []appconfigure.CommandSuggestion {
	seen := map[string]appconfigure.CommandSuggestion{}
	for _, command := range commands {
		if _, ok := seen[command.ID]; !ok {
			seen[command.ID] = command
		}
	}
	out := make([]appconfigure.CommandSuggestion, 0, len(seen))
	for _, command := range seen {
		out = append(out, command)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func dedupeInvariants(invariants []appconfigure.InvariantSuggestion) []appconfigure.InvariantSuggestion {
	seen := map[string]appconfigure.InvariantSuggestion{}
	for _, invariant := range invariants {
		if _, ok := seen[invariant.ID]; !ok {
			seen[invariant.ID] = invariant
		}
	}
	out := make([]appconfigure.InvariantSuggestion, 0, len(seen))
	for _, invariant := range seen {
		out = append(out, invariant)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
