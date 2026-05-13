package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	appconfig "github.com/nilstate/scafld/v2/internal/app/config"
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

func TestScannerTreatsAgentRulesAsConventionSurface(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "AGENTS.md", "# Agent rules\n")
	writeFile(t, root, "CLAUDE.md", "# Claude rules\n")
	writeFile(t, root, ".claude/rules/style.md", "# Style\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"AGENTS.md", "CLAUDE.md", ".claude/rules", ".claude/rules/style.md"} {
		if !hasEvidence(snapshot.Files, path) {
			t.Fatalf("evidence missing %s in %+v", path, snapshot.Files)
		}
	}
	if !hasInvariantID(snapshot.Invariants, "agent_guidance_alignment") {
		t.Fatalf("agent guidance invariant missing in %+v", snapshot.Invariants)
	}
	if !hasInvariantSource(snapshot.Invariants, "agent_guidance_alignment", ".claude/rules/style.md") {
		t.Fatalf("agent guidance invariant did not cite claude rules: %+v", snapshot.Invariants)
	}
}

func TestScannerSuggestsExecutionEnvironmentFromVersionManagers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "api/.ruby-version", "3.4.5\n")
	writeFile(t, root, "api/Gemfile", "gem 'rspec'\n")
	writeFile(t, root, ".tool-versions", "nodejs 24.0.0\npython 3.13.0\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution == nil {
		t.Fatalf("execution suggestion missing")
	}
	for _, want := range []string{"$HOME/.rbenv/shims", "$HOME/.asdf/shims", "$HOME/.local/share/mise/shims"} {
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
	if snapshot.Execution.Env["BUNDLE_GEMFILE"] != "api/Gemfile" {
		t.Fatalf("execution env = %+v", snapshot.Execution.Env)
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

func TestScannerDetectsProductAndDeploymentSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "package.json", `{"packageManager":"bun@1.2.0","scripts":{"check":"bun test && bun run typecheck","build":"vite build","typecheck":"tsc --noEmit"}}`)
	writeFile(t, root, "bun.lock", "lock\n")
	writeFile(t, root, "tsconfig.json", "{}\n")
	writeFile(t, root, "turbo.json", "{}\n")
	writeFile(t, root, "openapi.yaml", "openapi: 3.1.0\n")
	writeFile(t, root, "Dockerfile", "FROM scratch\n")
	writeFile(t, root, "vercel.json", "{}\n")
	writeFile(t, root, "pyproject.toml", "[project]\nname='app'\n[tool.pytest.ini_options]\n[tool.ruff]\n")
	writeFile(t, root, "uv.lock", "version = 1\n")
	writeFile(t, root, "Gemfile", "gem 'rails'\n")
	writeFile(t, root, "Gemfile.lock", "GEM\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for id, command := range map[string]string{
		"full_check":     "bun check",
		"node_build":     "bun build",
		"python_test":    "uv run pytest",
		"python_lint":    "uv run ruff check .",
		"ruby_test":      "bundle exec rails test",
		"node_typecheck": "bun typecheck",
	} {
		if !hasCommandSuggestion(snapshot.Commands, id, command) {
			t.Fatalf("missing command %s=%q in %+v", id, command, snapshot.Commands)
		}
	}
	for _, id := range []string{
		"node_lockfile_integrity",
		"typescript_boundary_integrity",
		"workspace_pipeline_integrity",
		"public_api_contract",
		"container_runtime_integrity",
		"deployment_surface_integrity",
		"python_lockfile_integrity",
		"ruby_lockfile_integrity",
	} {
		if !hasInvariantID(snapshot.Invariants, id) {
			t.Fatalf("missing invariant %s in %+v", id, snapshot.Invariants)
		}
	}
	if len(snapshot.Questions) != 0 {
		t.Fatalf("full_check should avoid command question: %+v", snapshot.Questions)
	}
}

func TestScannerDetectsNestedPackageSurfaces(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "api/go.mod", "module example.com/api\n")
	writeFile(t, root, "web/package.json", `{"packageManager":"pnpm@10.0.0","scripts":{"test":"vitest","lint":"eslint .","typecheck":"tsc --noEmit"}}`)
	writeFile(t, root, "web/pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
	writeFile(t, root, "worker/pyproject.toml", "[project]\nname='worker'\n[tool.pytest.ini_options]\n")
	writeFile(t, root, "crate/Cargo.toml", "[package]\nname='crate'\n")
	writeFile(t, root, "api/Gemfile", "gem 'rspec'\n")
	writeFile(t, root, "docker-compose.yml", "services: {}\n")
	writeFile(t, root, "Procfile", "web: bin/server\n")

	snapshot, err := Scanner{Root: root}.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for path := range map[string]bool{
		"api/go.mod":            true,
		"web/package.json":      true,
		"worker/pyproject.toml": true,
		"crate/Cargo.toml":      true,
		"api/Gemfile":           true,
	} {
		if !hasEvidence(snapshot.Files, path) {
			t.Fatalf("missing nested evidence %s in %+v", path, snapshot.Files)
		}
	}
	for id, command := range map[string]string{
		"go_test_api":        "(cd api && go test ./...)",
		"node_test_web":      "(cd web && pnpm test)",
		"node_typecheck_web": "(cd web && pnpm typecheck)",
		"python_test_worker": "(cd worker && python -m pytest)",
		"cargo_test_crate":   "(cd crate && cargo test)",
		"ruby_test_api":      "(cd api && bundle exec rspec)",
	} {
		if !hasCommandSuggestion(snapshot.Commands, id, command) {
			t.Fatalf("missing nested command %s=%q in %+v", id, command, snapshot.Commands)
		}
	}
	for _, id := range []string{"service_topology_integrity", "process_topology_integrity"} {
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

func TestScannerWarnsAboutIgnoredConfigKeys(t *testing.T) {
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
	if len(snapshot.Warnings) != 1 || snapshot.Warnings[0].ID != "ignored_config_keys" {
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

func hasCommandID(commands []appconfig.CommandSuggestion, id string) bool {
	for _, command := range commands {
		if command.ID == id {
			return true
		}
	}
	return false
}

func hasInvariantID(invariants []appconfig.InvariantSuggestion, id string) bool {
	for _, invariant := range invariants {
		if invariant.ID == id {
			return true
		}
	}
	return false
}

func hasCommandSuggestion(commands []appconfig.CommandSuggestion, id string, command string) bool {
	for _, candidate := range commands {
		if candidate.ID == id && candidate.Command == command {
			return true
		}
	}
	return false
}

func hasEvidence(files []appconfig.Evidence, path string) bool {
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

func hasInvariantSource(invariants []appconfig.InvariantSuggestion, id string, source string) bool {
	for _, invariant := range invariants {
		if invariant.ID != id {
			continue
		}
		for _, candidate := range invariant.Sources {
			if candidate == source {
				return true
			}
		}
	}
	return false
}
