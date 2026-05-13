package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	appconfig "github.com/nilstate/scafld/v2/internal/app/config"
)

// Scanner inspects a workspace for config-relevant evidence.
type Scanner struct {
	Root string
}

// Scan returns repo evidence without mutating the workspace.
func (s Scanner) Scan(ctx context.Context) (appconfig.Snapshot, error) {
	root := s.Root
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return appconfig.Snapshot{}, fmt.Errorf("resolve root: %w", err)
	}
	snapshot := appconfig.Snapshot{Root: abs}
	for _, candidate := range fileCandidates() {
		if err := ctx.Err(); err != nil {
			return appconfig.Snapshot{}, err
		}
		if exists(filepath.Join(abs, filepath.FromSlash(candidate.Path))) {
			snapshot.Files = append(snapshot.Files, candidate)
		}
	}
	snapshot.Files = append(snapshot.Files, nestedPackageEvidence(abs)...)
	snapshot.Files = append(snapshot.Files, toolchainEvidence(abs)...)
	snapshot.Files = append(snapshot.Files, agentRuleEvidence(abs)...)
	workflowFiles, err := globRel(abs, ".github/workflows/*")
	if err != nil {
		return appconfig.Snapshot{}, err
	}
	for _, rel := range workflowFiles {
		if strings.HasSuffix(rel, ".yml") || strings.HasSuffix(rel, ".yaml") {
			snapshot.Files = append(snapshot.Files, appconfig.Evidence{Path: rel, Role: "ci_workflow"})
		}
	}
	snapshot.Commands = commandSuggestions(abs)
	snapshot.Invariants = invariantSuggestions(abs, snapshot.Files)
	snapshot.Execution = executionSuggestion(abs)
	snapshot.Warnings = ignoredConfigWarnings(abs)
	snapshot.Questions = openQuestions(snapshot)
	return snapshot, nil
}

func fileCandidates() []appconfig.Evidence {
	return []appconfig.Evidence{
		{Path: "AGENTS.md", Role: "agent_contract"},
		{Path: "CLAUDE.md", Role: "claude_agent_contract"},
		{Path: "README.md", Role: "project_overview"},
		{Path: "Makefile", Role: "command_surface"},
		{Path: "justfile", Role: "command_surface"},
		{Path: "Taskfile.yml", Role: "command_surface"},
		{Path: "Taskfile.yaml", Role: "command_surface"},
		{Path: "go.mod", Role: "go_module"},
		{Path: "go.sum", Role: "go_lockfile"},
		{Path: "package.json", Role: "node_package"},
		{Path: "package-lock.json", Role: "node_lockfile"},
		{Path: "pnpm-lock.yaml", Role: "node_lockfile"},
		{Path: "yarn.lock", Role: "node_lockfile"},
		{Path: "bun.lock", Role: "node_lockfile"},
		{Path: "bun.lockb", Role: "node_lockfile"},
		{Path: "pnpm-workspace.yaml", Role: "node_workspace"},
		{Path: "tsconfig.json", Role: "typescript_config"},
		{Path: "turbo.json", Role: "workspace_pipeline"},
		{Path: "nx.json", Role: "workspace_pipeline"},
		{Path: "pyproject.toml", Role: "python_package"},
		{Path: "uv.lock", Role: "python_lockfile"},
		{Path: "poetry.lock", Role: "python_lockfile"},
		{Path: "requirements.txt", Role: "python_requirements"},
		{Path: "Cargo.toml", Role: "rust_package"},
		{Path: "Cargo.lock", Role: "rust_lockfile"},
		{Path: "Gemfile", Role: "ruby_package"},
		{Path: "Gemfile.lock", Role: "ruby_lockfile"},
		{Path: "config/routes.rb", Role: "rails_routes"},
		{Path: "openapi.yaml", Role: "public_api_schema"},
		{Path: "openapi.yml", Role: "public_api_schema"},
		{Path: "openapi.json", Role: "public_api_schema"},
		{Path: "Dockerfile", Role: "container_runtime"},
		{Path: "docker-compose.yml", Role: "service_topology"},
		{Path: "docker-compose.yaml", Role: "service_topology"},
		{Path: "compose.yml", Role: "service_topology"},
		{Path: "compose.yaml", Role: "service_topology"},
		{Path: "Procfile", Role: "process_topology"},
		{Path: "vercel.json", Role: "deployment_surface"},
		{Path: "netlify.toml", Role: "deployment_surface"},
		{Path: "wrangler.toml", Role: "deployment_surface"},
		{Path: "internal/arch/architecture_test.go", Role: "architecture_gate"},
	}
}

func nestedPackageEvidence(root string) []appconfig.Evidence {
	var evidence []appconfig.Evidence
	for _, item := range []struct {
		name string
		role string
	}{
		{"go.mod", "nested_go_module"},
		{"go.sum", "nested_go_lockfile"},
		{"package.json", "nested_node_package"},
		{"tsconfig.json", "nested_typescript_config"},
		{"pyproject.toml", "nested_python_package"},
		{"Cargo.toml", "nested_rust_package"},
		{"Gemfile", "nested_ruby_package"},
		{"Gemfile.lock", "nested_ruby_lockfile"},
		{"Dockerfile", "nested_container_runtime"},
	} {
		for _, rel := range oneLevelMatches(root, item.name) {
			evidence = append(evidence, appconfig.Evidence{Path: rel, Role: item.role})
		}
	}
	return evidence
}

func agentRuleEvidence(root string) []appconfig.Evidence {
	var evidence []appconfig.Evidence
	rules := filepath.Join(root, ".claude", "rules")
	info, err := os.Stat(rules)
	if err != nil {
		return nil
	}
	evidence = append(evidence, appconfig.Evidence{Path: ".claude/rules", Role: "claude_rules"})
	if !info.IsDir() {
		return evidence
	}
	_ = filepath.WalkDir(rules, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		evidence = append(evidence, appconfig.Evidence{Path: filepath.ToSlash(rel), Role: "claude_rules"})
		return nil
	})
	return evidence
}

func toolchainEvidence(root string) []appconfig.Evidence {
	var evidence []appconfig.Evidence
	if exists(filepath.Join(root, ".ruby-version")) {
		evidence = append(evidence, appconfig.Evidence{Path: ".ruby-version", Role: "ruby_version"})
	}
	for _, rel := range oneLevelMatches(root, ".ruby-version") {
		evidence = append(evidence, appconfig.Evidence{Path: rel, Role: "ruby_version"})
	}
	if exists(filepath.Join(root, ".tool-versions")) {
		evidence = append(evidence, appconfig.Evidence{Path: ".tool-versions", Role: "tool_versions"})
	}
	for _, rel := range oneLevelMatches(root, ".tool-versions") {
		evidence = append(evidence, appconfig.Evidence{Path: rel, Role: "tool_versions"})
	}
	if exists(filepath.Join(root, "db", "migrate")) {
		evidence = append(evidence, appconfig.Evidence{Path: "db/migrate", Role: "migration_surface"})
	}
	return evidence
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
