package configure

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfigure "github.com/nilstate/scafld/v2/internal/app/configure"
)

func TestScannerFindsCommandsAndInvariants(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "Makefile", "check:\n\tgo test ./...\n")
	writeFile(t, root, "go.mod", "module example.com/project\n")
	writeFile(t, root, "internal/arch/architecture_test.go", "package arch\n")
	writeFile(t, root, ".github/workflows/ci.yml", "name: ci\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !hasCommandID(snapshot.Commands, "full_check") {
		t.Fatalf("commands = %+v", snapshot.Commands)
	}
	for _, want := range []string{"architecture_boundaries", "ci_must_pass", "go_module_integrity"} {
		if !hasInvariantID(snapshot.Invariants, want) {
			t.Fatalf("missing invariant %s in %+v", want, snapshot.Invariants)
		}
	}
	if len(snapshot.Questions) != 0 {
		t.Fatalf("questions = %+v", snapshot.Questions)
	}
}

func TestScannerSuggestsExecutionEnvironmentFromRubyVersionManagers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "api/.ruby-version", "3.4.5\n")
	writeFile(t, root, ".tool-versions", "ruby 3.4.5\nnodejs 24.0.0\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution == nil {
		t.Fatalf("execution suggestion missing")
	}
	for _, want := range []string{"$HOME/.rbenv/shims", "$HOME/.asdf/shims"} {
		if !contains(snapshot.Execution.PathPrepend, want) {
			t.Fatalf("path_prepend = %+v, missing %s", snapshot.Execution.PathPrepend, want)
		}
	}
	for _, want := range []string{"api/.ruby-version", ".tool-versions"} {
		if !contains(snapshot.Execution.Sources, want) {
			t.Fatalf("sources = %+v, missing %s", snapshot.Execution.Sources, want)
		}
	}
	for _, want := range []string{"api/.ruby-version", ".tool-versions"} {
		if !hasEvidence(snapshot.Files, want) {
			t.Fatalf("evidence missing %s in %+v", want, snapshot.Files)
		}
	}
}

func TestScannerDetectsRealWorldCommandSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "package.json", `{"packageManager":"pnpm@10.0.0","scripts":{"test":"vitest","lint":"eslint .","typecheck":"tsc --noEmit"}}`)
	writeFile(t, root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	writeFile(t, root, "pyproject.toml", "[project]\nname='app'\n[tool.pytest.ini_options]\n[tool.ruff]\n")
	writeFile(t, root, "Cargo.toml", "[package]\nname='crate'\n")
	writeFile(t, root, "Gemfile", "gem 'rspec'\n")
	writeFile(t, root, "justfile", "check:\n\tgo test ./...\n")
	writeFile(t, root, "Taskfile.yml", "tasks:\n  test:\n    cmds:\n      - go test ./...\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for id, command := range map[string]string{
		"node_test":      "pnpm test",
		"node_lint":      "pnpm lint",
		"node_typecheck": "pnpm typecheck",
		"python_test":    "python -m pytest",
		"python_lint":    "python -m ruff check .",
		"cargo_test":     "cargo test",
		"ruby_test":      "bundle exec rspec",
		"just_check":     "just check",
		"task_test":      "task test",
	} {
		if !hasCommandSuggestion(snapshot.Commands, id, command) {
			t.Fatalf("missing command %s=%q in %+v", id, command, snapshot.Commands)
		}
	}
	for _, id := range []string{"package_script_integrity", "python_environment_integrity", "ruby_bundle_integrity", "rust_crate_integrity"} {
		if !hasInvariantID(snapshot.Invariants, id) {
			t.Fatalf("missing invariant %s in %+v", id, snapshot.Invariants)
		}
	}
}

func TestScannerAsksWhenNoFullCheckExists(t *testing.T) {
	t.Parallel()

	snapshot, err := Scanner{Root: t.TempDir()}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Questions) == 0 {
		t.Fatalf("questions = %+v, want missing full check question", snapshot.Questions)
	}
}

func TestScannerWarnsAboutLegacyIgnoredConfigKeys(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, ".scafld/config.yaml", `
version: "1.0"
validation:
  per_phase: []
rubric:
  threshold: 7
tech_stack:
  backend: go
`)
	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Warnings) != 1 || snapshot.Warnings[0].ID != "legacy_ignored_config_keys" {
		t.Fatalf("warnings = %+v", snapshot.Warnings)
	}
	for _, want := range []string{"validation", "rubric", "tech_stack"} {
		if !strings.Contains(snapshot.Warnings[0].Message, want) {
			t.Fatalf("warning %q does not mention %s", snapshot.Warnings[0].Message, want)
		}
	}
}

func writeFile(t *testing.T, root string, rel string, text string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasCommandID(commands []appconfigure.CommandSuggestion, id string) bool {
	for _, command := range commands {
		if command.ID == id {
			return true
		}
	}
	return false
}

func hasInvariantID(invariants []appconfigure.InvariantSuggestion, id string) bool {
	for _, invariant := range invariants {
		if invariant.ID == id {
			return true
		}
	}
	return false
}

func hasCommandSuggestion(commands []appconfigure.CommandSuggestion, id string, command string) bool {
	for _, candidate := range commands {
		if candidate.ID == id && candidate.Command == command {
			return true
		}
	}
	return false
}

func hasEvidence(files []appconfigure.Evidence, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
