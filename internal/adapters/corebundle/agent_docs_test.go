package corebundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitAgentDocsCreatesRootDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	result, err := InitAgentDocs(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("created = %v, want two root docs", result.Created)
	}
	for _, name := range agentDocFiles {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		if strings.Contains(text, "scafld:contract") || !strings.Contains(text, "scafld") || !strings.HasPrefix(text, "# scafld") {
			t.Fatalf("%s missing plain scafld section:\n%s", name, text)
		}
	}
}

func TestAgentDocResetCopiesMatchRootTemplates(t *testing.T) {
	t.Parallel()

	for _, name := range agentDocFiles {
		rootTemplate, err := assets.ReadFile("assets/agentdocs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		coreTemplate, err := assets.ReadFile("assets/core/agentdocs/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(rootTemplate) != string(coreTemplate) {
			t.Fatalf("%s root template and core reset copy drifted", name)
		}
	}
}

func TestInitAgentDocsPrependsExistingDocs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(path, []byte("# Project Agent Rules\n\nKeep this project note.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := InitAgentDocs(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# scafld Agent Contract") {
		t.Fatalf("scafld section was not prepended:\n%s", text)
	}
	if !strings.Contains(text, "# Project Agent Rules") || !strings.Contains(text, "Keep this project note.") {
		t.Fatalf("existing project content was not preserved:\n%s", text)
	}
}

func TestRefreshAgentDocsUpdatesOnlyScafldSection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	agents := filepath.Join(root, "AGENTS.md")
	claude := filepath.Join(root, "CLAUDE.md")
	if err := os.WriteFile(agents, []byte("# scafld Agent Contract\n\nstale\n\n# Project Rules\n\nproject note\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claude, []byte("# User Claude Notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := RefreshAgentDocs(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 1 || result.Updated[0] != "AGENTS.md" {
		t.Fatalf("updated = %v, want AGENTS.md only", result.Updated)
	}
	data, err := os.ReadFile(agents)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, "stale") || !strings.Contains(text, "project note") || !strings.Contains(text, "scafld Agent Contract") {
		t.Fatalf("scafld section was not refreshed correctly:\n%s", text)
	}
	data, err = os.ReadFile(claude)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# User Claude Notes\n" {
		t.Fatalf("unmarked doc should not be touched:\n%s", data)
	}
}

func TestRefreshAgentDocsSkipsCurrentChecksum(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if _, err := InitAgentDocs(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	result, err := RefreshAgentDocs(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Updated) != 0 {
		t.Fatalf("updated = %v, want current docs skipped", result.Updated)
	}
}

func TestRefreshAgentDocsRejectsDuplicateScafldHeading(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# scafld Agent Contract\n\none\n\n# scafld Agent Contract\n\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshAgentDocs(t.Context(), root); err == nil {
		t.Fatal("RefreshAgentDocs() error = nil, want duplicate heading error")
	}
}
