package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	appconfig "github.com/nilstate/scafld/v2/internal/app/config"
)

func commandSuggestions(root string) []appconfig.CommandSuggestion {
	var commands []appconfig.CommandSuggestion
	if makefile := readText(filepath.Join(root, "Makefile")); makefile != "" {
		if hasMakeTarget(makefile, "check") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "full_check", Command: "make check", Sources: []string{"Makefile"}})
		}
		if hasMakeTarget(makefile, "test") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "test", Command: "make test", Sources: []string{"Makefile"}})
		}
	}
	if justfile := readText(filepath.Join(root, "justfile")); justfile != "" {
		if hasMakeTarget(justfile, "check") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "just_check", Command: "just check", Sources: []string{"justfile"}})
		}
		if hasMakeTarget(justfile, "test") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "just_test", Command: "just test", Sources: []string{"justfile"}})
		}
	}
	if exists(filepath.Join(root, "Taskfile.yml")) || exists(filepath.Join(root, "Taskfile.yaml")) {
		taskfile := firstExisting(root, "Taskfile.yml", "Taskfile.yaml")
		text := readText(filepath.Join(root, taskfile))
		if hasYAMLTask(text, "check") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "task_check", Command: "task check", Sources: []string{taskfile}})
		}
		if hasYAMLTask(text, "test") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "task_test", Command: "task test", Sources: []string{taskfile}})
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
				commands = append(commands, appconfig.CommandSuggestion{ID: "node_test", Command: manager + " test", Sources: []string{"package.json:scripts.test"}})
			}
			if _, ok := pkg.Scripts["lint"]; ok {
				commands = append(commands, appconfig.CommandSuggestion{ID: "node_lint", Command: nodeRun(manager, "lint"), Sources: []string{"package.json:scripts.lint"}})
			}
			if _, ok := pkg.Scripts["typecheck"]; ok {
				commands = append(commands, appconfig.CommandSuggestion{ID: "node_typecheck", Command: nodeRun(manager, "typecheck"), Sources: []string{"package.json:scripts.typecheck"}})
			}
		}
	}
	if readText(filepath.Join(root, "go.mod")) != "" {
		commands = append(commands, appconfig.CommandSuggestion{ID: "go_test", Command: "go test ./...", Sources: []string{"go.mod"}})
	}
	if readText(filepath.Join(root, "Cargo.toml")) != "" {
		commands = append(commands, appconfig.CommandSuggestion{ID: "cargo_test", Command: "cargo test", Sources: []string{"Cargo.toml"}})
	}
	if pyproject := readText(filepath.Join(root, "pyproject.toml")); pyproject != "" {
		if strings.Contains(pyproject, "pytest") || exists(filepath.Join(root, "tests")) {
			commands = append(commands, appconfig.CommandSuggestion{ID: "python_test", Command: "python -m pytest", Sources: []string{"pyproject.toml"}})
		}
		if strings.Contains(pyproject, "ruff") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "python_lint", Command: "python -m ruff check .", Sources: []string{"pyproject.toml"}})
		}
	}
	if gemfile := readText(filepath.Join(root, "Gemfile")); gemfile != "" {
		if strings.Contains(gemfile, "rspec") || exists(filepath.Join(root, "spec")) {
			commands = append(commands, appconfig.CommandSuggestion{ID: "ruby_test", Command: "bundle exec rspec", Sources: []string{"Gemfile"}})
		}
	}
	commands = append(commands, nestedCommandSuggestions(root)...)
	return dedupeCommands(commands)
}

func nestedCommandSuggestions(root string) []appconfig.CommandSuggestion {
	var commands []appconfig.CommandSuggestion
	for _, rel := range oneLevelMatches(root, "go.mod") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		commands = append(commands, appconfig.CommandSuggestion{ID: "go_test_" + idPart(dir), Command: "(cd " + dir + " && go test ./...)", Sources: []string{rel}})
	}
	for _, rel := range oneLevelMatches(root, "package.json") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		text := readText(filepath.Join(root, filepath.FromSlash(rel)))
		var pkg struct {
			Scripts        map[string]string `json:"scripts"`
			PackageManager string            `json:"packageManager"`
		}
		if json.Unmarshal([]byte(text), &pkg) != nil {
			continue
		}
		manager := nodePackageManager(filepath.Join(root, filepath.FromSlash(dir)), pkg.PackageManager)
		if _, ok := pkg.Scripts["test"]; ok {
			commands = append(commands, appconfig.CommandSuggestion{ID: "node_test_" + idPart(dir), Command: "(cd " + dir + " && " + manager + " test)", Sources: []string{rel + ":scripts.test"}})
		}
		if _, ok := pkg.Scripts["lint"]; ok {
			commands = append(commands, appconfig.CommandSuggestion{ID: "node_lint_" + idPart(dir), Command: "(cd " + dir + " && " + nodeRun(manager, "lint") + ")", Sources: []string{rel + ":scripts.lint"}})
		}
		if _, ok := pkg.Scripts["typecheck"]; ok {
			commands = append(commands, appconfig.CommandSuggestion{ID: "node_typecheck_" + idPart(dir), Command: "(cd " + dir + " && " + nodeRun(manager, "typecheck") + ")", Sources: []string{rel + ":scripts.typecheck"}})
		}
	}
	for _, rel := range oneLevelMatches(root, "pyproject.toml") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		text := readText(filepath.Join(root, filepath.FromSlash(rel)))
		if strings.Contains(text, "pytest") || exists(filepath.Join(root, filepath.FromSlash(dir), "tests")) {
			commands = append(commands, appconfig.CommandSuggestion{ID: "python_test_" + idPart(dir), Command: "(cd " + dir + " && python -m pytest)", Sources: []string{rel}})
		}
		if strings.Contains(text, "ruff") {
			commands = append(commands, appconfig.CommandSuggestion{ID: "python_lint_" + idPart(dir), Command: "(cd " + dir + " && python -m ruff check .)", Sources: []string{rel}})
		}
	}
	for _, rel := range oneLevelMatches(root, "Cargo.toml") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		commands = append(commands, appconfig.CommandSuggestion{ID: "cargo_test_" + idPart(dir), Command: "(cd " + dir + " && cargo test)", Sources: []string{rel}})
	}
	for _, rel := range oneLevelMatches(root, "Gemfile") {
		dir := filepath.ToSlash(filepath.Dir(rel))
		gemfile := readText(filepath.Join(root, filepath.FromSlash(rel)))
		if strings.Contains(gemfile, "rspec") || exists(filepath.Join(root, filepath.FromSlash(dir), "spec")) {
			commands = append(commands, appconfig.CommandSuggestion{ID: "ruby_test_" + idPart(dir), Command: "(cd " + dir + " && bundle exec rspec)", Sources: []string{rel}})
		}
	}
	return commands
}

func invariantSuggestions(root string, files []appconfig.Evidence) []appconfig.InvariantSuggestion {
	var invariants []appconfig.InvariantSuggestion
	if evidenceRole(files, "architecture_gate") {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "architecture_boundaries",
			Description: "Preserve the architecture boundaries enforced by the repo's architecture tests.",
			Sources:     []string{"internal/arch/architecture_test.go"},
		})
	}
	if evidenceRole(files, "ci_workflow") {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "ci_must_pass",
			Description: "Changes should preserve the committed CI workflow expectations.",
			Sources:     pathsByRole(files, "ci_workflow"),
		})
	}
	if readText(filepath.Join(root, "go.mod")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "go_module_integrity",
			Description: "Keep the Go module buildable and avoid unchecked dependency or module-path drift.",
			Sources:     []string{"go.mod"},
		})
	}
	if readText(filepath.Join(root, "package.json")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "package_script_integrity",
			Description: "Keep package scripts and generated package metadata aligned with release behavior.",
			Sources:     []string{"package.json"},
		})
	}
	if readText(filepath.Join(root, "pnpm-workspace.yaml")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "workspace_package_boundaries",
			Description: "Preserve workspace package boundaries and avoid undeclared cross-package coupling.",
			Sources:     []string{"pnpm-workspace.yaml"},
		})
	}
	if readText(filepath.Join(root, "pyproject.toml")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "python_environment_integrity",
			Description: "Keep Python project metadata, runtime dependencies, and validation commands aligned.",
			Sources:     []string{"pyproject.toml"},
		})
	}
	if readText(filepath.Join(root, "Gemfile")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "ruby_bundle_integrity",
			Description: "Keep Ruby version, Bundler context, and gem dependencies aligned with validation commands.",
			Sources:     []string{"Gemfile"},
		})
	}
	if readText(filepath.Join(root, "Cargo.toml")) != "" {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "rust_crate_integrity",
			Description: "Keep Cargo manifests, lockfiles, and crate validation aligned.",
			Sources:     []string{"Cargo.toml"},
		})
	}
	if exists(filepath.Join(root, "db", "migrate")) {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "migration_safety",
			Description: "Schema migrations require explicit rollback thinking and public data-safety review.",
			Sources:     []string{"db/migrate"},
		})
	}
	if paths := pathsByRole(files, "service_topology"); len(paths) > 0 {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "service_topology_integrity",
			Description: "Keep service topology and local orchestration files aligned with code changes.",
			Sources:     paths,
		})
	}
	if paths := pathsByRole(files, "process_topology"); len(paths) > 0 {
		invariants = append(invariants, appconfig.InvariantSuggestion{
			ID:          "process_topology_integrity",
			Description: "Keep declared process entrypoints aligned with runtime behavior.",
			Sources:     paths,
		})
	}
	return dedupeInvariants(invariants)
}

func executionSuggestion(root string) *appconfig.ExecutionSuggestion {
	var paths []string
	var sources []string
	env := map[string]string{}
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
	if !exists(filepath.Join(root, "Gemfile")) {
		gemfiles := oneLevelMatches(root, "Gemfile")
		if len(gemfiles) == 1 {
			env["BUNDLE_GEMFILE"] = gemfiles[0]
			sources = dedupeStrings(append(sources, gemfiles[0]))
		}
	}
	if len(paths) == 0 && len(env) == 0 {
		return nil
	}
	return &appconfig.ExecutionSuggestion{PathPrepend: paths, Env: env, Sources: sources}
}

func openQuestions(snapshot appconfig.Snapshot) []appconfig.Question {
	var questions []appconfig.Question
	if !hasCommand(snapshot.Commands, "full_check") {
		questions = append(questions, appconfig.Question{
			Question: "What single command is authoritative before release or commit?",
			Reason:   "No `make check` target was detected.",
		})
	}
	if len(snapshot.Invariants) == 0 {
		questions = append(questions, appconfig.Question{
			Question: "Which project-specific invariants must every spec consider?",
			Reason:   "No architecture gate, CI workflow, or recognized package manifest implied a concrete invariant.",
		})
	}
	return questions
}

func ignoredConfigWarnings(root string) []appconfig.Warning {
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
	return []appconfig.Warning{{
		ID:      "ignored_config_keys",
		Message: "The current .scafld/config.yaml contains keys the Go runtime does not read: " + strings.Join(found, ", ") + ". Move real policy into invariants, specs, acceptance criteria, or review passes before deleting stale keys.",
		Sources: []string{".scafld/config.yaml"},
	}}
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

func idPart(path string) string {
	text := regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(path, "_")
	return strings.Trim(text, "_")
}

func evidenceRole(files []appconfig.Evidence, role string) bool {
	return len(pathsByRole(files, role)) > 0
}

func pathsByRole(files []appconfig.Evidence, role string) []string {
	var paths []string
	for _, file := range files {
		if file.Role == role {
			paths = append(paths, file.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func hasCommand(commands []appconfig.CommandSuggestion, id string) bool {
	for _, command := range commands {
		if command.ID == id {
			return true
		}
	}
	return false
}

func dedupeCommands(commands []appconfig.CommandSuggestion) []appconfig.CommandSuggestion {
	seen := map[string]appconfig.CommandSuggestion{}
	for _, command := range commands {
		if _, ok := seen[command.ID]; !ok {
			seen[command.ID] = command
		}
	}
	out := make([]appconfig.CommandSuggestion, 0, len(seen))
	for _, command := range seen {
		out = append(out, command)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func dedupeInvariants(invariants []appconfig.InvariantSuggestion) []appconfig.InvariantSuggestion {
	seen := map[string]appconfig.InvariantSuggestion{}
	for _, invariant := range invariants {
		if _, ok := seen[invariant.ID]; !ok {
			seen[invariant.ID] = invariant
		}
	}
	out := make([]appconfig.InvariantSuggestion, 0, len(seen))
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
