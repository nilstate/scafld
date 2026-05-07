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
	snapshot.Files = append(snapshot.Files, toolchainEvidence(abs)...)
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
	snapshot.Execution = executionSuggestion(abs)
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

func toolchainEvidence(root string) []appconfigure.Evidence {
	var evidence []appconfigure.Evidence
	if exists(filepath.Join(root, ".ruby-version")) {
		evidence = append(evidence, appconfigure.Evidence{Path: ".ruby-version", Role: "ruby_version"})
	}
	for _, rel := range oneLevelMatches(root, ".ruby-version") {
		evidence = append(evidence, appconfigure.Evidence{Path: rel, Role: "ruby_version"})
	}
	if exists(filepath.Join(root, ".tool-versions")) {
		evidence = append(evidence, appconfigure.Evidence{Path: ".tool-versions", Role: "tool_versions"})
	}
	for _, rel := range oneLevelMatches(root, ".tool-versions") {
		evidence = append(evidence, appconfigure.Evidence{Path: rel, Role: "tool_versions"})
	}
	if exists(filepath.Join(root, "db", "migrate")) {
		evidence = append(evidence, appconfigure.Evidence{Path: "db/migrate", Role: "migration_surface"})
	}
	return evidence
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
	if justfile := readText(filepath.Join(root, "justfile")); justfile != "" {
		if hasMakeTarget(justfile, "check") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "just_check", Command: "just check", Sources: []string{"justfile"}})
		}
		if hasMakeTarget(justfile, "test") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "just_test", Command: "just test", Sources: []string{"justfile"}})
		}
	}
	if exists(filepath.Join(root, "Taskfile.yml")) || exists(filepath.Join(root, "Taskfile.yaml")) {
		taskfile := firstExisting(root, "Taskfile.yml", "Taskfile.yaml")
		text := readText(filepath.Join(root, taskfile))
		if hasYAMLTask(text, "check") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "task_check", Command: "task check", Sources: []string{taskfile}})
		}
		if hasYAMLTask(text, "test") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "task_test", Command: "task test", Sources: []string{taskfile}})
		}
	}
	if packageJSON := readText(filepath.Join(root, "package.json")); packageJSON != "" {
		var pkg struct {
			Scripts        map[string]string `json:"scripts"`
			PackageManager string            `json:"packageManager"`
		}
		if json.Unmarshal([]byte(packageJSON), &pkg) == nil {
			manager := nodePackageManager(root, pkg.PackageManager)
			if _, ok := pkg.Scripts["test"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "node_test", Command: manager + " test", Sources: []string{"package.json:scripts.test"}})
			}
			if _, ok := pkg.Scripts["lint"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "node_lint", Command: nodeRun(manager, "lint"), Sources: []string{"package.json:scripts.lint"}})
			}
			if _, ok := pkg.Scripts["typecheck"]; ok {
				commands = append(commands, appconfigure.CommandSuggestion{ID: "node_typecheck", Command: nodeRun(manager, "typecheck"), Sources: []string{"package.json:scripts.typecheck"}})
			}
		}
	}
	if readText(filepath.Join(root, "go.mod")) != "" {
		commands = append(commands, appconfigure.CommandSuggestion{ID: "go_test", Command: "go test ./...", Sources: []string{"go.mod"}})
	}
	if readText(filepath.Join(root, "Cargo.toml")) != "" {
		commands = append(commands, appconfigure.CommandSuggestion{ID: "cargo_test", Command: "cargo test", Sources: []string{"Cargo.toml"}})
	}
	if pyproject := readText(filepath.Join(root, "pyproject.toml")); pyproject != "" {
		if strings.Contains(pyproject, "pytest") || exists(filepath.Join(root, "tests")) {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "python_test", Command: "python -m pytest", Sources: []string{"pyproject.toml"}})
		}
		if strings.Contains(pyproject, "ruff") {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "python_lint", Command: "python -m ruff check .", Sources: []string{"pyproject.toml"}})
		}
	}
	if gemfile := readText(filepath.Join(root, "Gemfile")); gemfile != "" {
		if strings.Contains(gemfile, "rspec") || exists(filepath.Join(root, "spec")) {
			commands = append(commands, appconfigure.CommandSuggestion{ID: "ruby_test", Command: "bundle exec rspec", Sources: []string{"Gemfile"}})
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
	if readText(filepath.Join(root, "pnpm-workspace.yaml")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "workspace_package_boundaries",
			Description: "Preserve workspace package boundaries and avoid undeclared cross-package coupling.",
			Sources:     []string{"pnpm-workspace.yaml"},
		})
	}
	if readText(filepath.Join(root, "pyproject.toml")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "python_environment_integrity",
			Description: "Keep Python project metadata, runtime dependencies, and validation commands aligned.",
			Sources:     []string{"pyproject.toml"},
		})
	}
	if readText(filepath.Join(root, "Gemfile")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "ruby_bundle_integrity",
			Description: "Keep Ruby version, Bundler context, and gem dependencies aligned with validation commands.",
			Sources:     []string{"Gemfile"},
		})
	}
	if readText(filepath.Join(root, "Cargo.toml")) != "" {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "rust_crate_integrity",
			Description: "Keep Cargo manifests, lockfiles, and crate validation aligned.",
			Sources:     []string{"Cargo.toml"},
		})
	}
	if exists(filepath.Join(root, "db", "migrate")) {
		invariants = append(invariants, appconfigure.InvariantSuggestion{
			ID:          "migration_safety",
			Description: "Schema migrations require explicit rollback thinking and public data-safety review.",
			Sources:     []string{"db/migrate"},
		})
	}
	return dedupeInvariants(invariants)
}

func executionSuggestion(root string) *appconfigure.ExecutionSuggestion {
	var paths []string
	var sources []string
	if exists(filepath.Join(root, ".ruby-version")) {
		paths = append(paths, "$HOME/.rbenv/shims")
		sources = append(sources, ".ruby-version")
	}
	for _, rel := range oneLevelMatches(root, ".ruby-version") {
		paths = append(paths, "$HOME/.rbenv/shims")
		sources = append(sources, rel)
	}
	if toolVersions := readText(filepath.Join(root, ".tool-versions")); strings.Contains(toolVersions, "ruby ") || strings.Contains(toolVersions, "ruby\t") {
		paths = append(paths, "$HOME/.asdf/shims")
		sources = append(sources, ".tool-versions")
	}
	for _, rel := range oneLevelMatches(root, ".tool-versions") {
		text := readText(filepath.Join(root, filepath.FromSlash(rel)))
		if strings.Contains(text, "ruby ") || strings.Contains(text, "ruby\t") {
			paths = append(paths, "$HOME/.asdf/shims")
			sources = append(sources, rel)
		}
	}
	paths = dedupeStrings(paths)
	sources = dedupeStrings(sources)
	if len(paths) == 0 {
		return nil
	}
	return &appconfigure.ExecutionSuggestion{PathPrepend: paths, Sources: sources}
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

func oneLevelMatches(root string, name string) []string {
	matches, err := filepath.Glob(filepath.Join(root, "*", name))
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		rel, err := filepath.Rel(root, match)
		if err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
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

func hasYAMLTask(text string, target string) bool {
	return regexp.MustCompile(`(?m)^\s+`+regexp.QuoteMeta(target)+`:`).MatchString(text) ||
		regexp.MustCompile(`(?m)^`+regexp.QuoteMeta(target)+`:`).MatchString(text)
}

func firstExisting(root string, candidates ...string) string {
	for _, candidate := range candidates {
		if exists(filepath.Join(root, candidate)) {
			return candidate
		}
	}
	return ""
}

func nodePackageManager(root string, declared string) string {
	name := strings.TrimSpace(strings.Split(declared, "@")[0])
	switch name {
	case "pnpm", "yarn", "bun", "npm":
		return name
	}
	switch {
	case exists(filepath.Join(root, "pnpm-lock.yaml")):
		return "pnpm"
	case exists(filepath.Join(root, "yarn.lock")):
		return "yarn"
	case exists(filepath.Join(root, "bun.lockb")) || exists(filepath.Join(root, "bun.lock")):
		return "bun"
	default:
		return "npm"
	}
}

func nodeRun(manager string, script string) string {
	if manager == "npm" {
		return "npm run " + script
	}
	return manager + " " + script
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

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
