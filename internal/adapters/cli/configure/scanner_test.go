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
