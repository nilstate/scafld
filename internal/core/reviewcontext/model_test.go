package reviewcontext

import (
	"strings"
	"testing"
)

func TestRenderMarkdownIsDeterministic(t *testing.T) {
	t.Parallel()

	packet := Packet{
		TaskID: "task",
		Title:  "Task",
		Status: "review",
		Sections: []Section{
			{Key: "b", Title: "B", Order: 20, Body: "second"},
			{Key: "a", Title: "A", Order: 10, Body: "first"},
		},
	}
	got := RenderMarkdown(packet, Options{})
	if strings.Index(got, "## A") > strings.Index(got, "## B") {
		t.Fatalf("sections not ordered:\n%s", got)
	}
	again := RenderMarkdown(Packet{TaskID: "task", Title: "Task", Status: "review", Sections: []Section{
		{Key: "a", Title: "A", Order: 10, Body: "first"},
		{Key: "b", Title: "B", Order: 20, Body: "second"},
	}}, Options{})
	if got != again {
		t.Fatalf("render drifted:\n--- got ---\n%s\n--- again ---\n%s", got, again)
	}
}

func TestRenderMarkdownIncludesSourceProvenance(t *testing.T) {
	t.Parallel()

	source := SourceForContent("file", "AGENTS.md", []byte("contract"))
	got := RenderMarkdown(Packet{TaskID: "task", Sections: []Section{{
		Key:     "contract",
		Title:   "Contract",
		Order:   10,
		Body:    "contract",
		Sources: []Source{source, source},
	}}}, Options{})
	if strings.Count(got, "AGENTS.md") != 1 || !strings.Contains(got, source.SHA256) || !strings.Contains(got, "bytes=8") {
		t.Fatalf("source provenance missing or duplicated:\n%s", got)
	}
}

func TestRenderMarkdownTruncatesWithOmissionCount(t *testing.T) {
	t.Parallel()

	got := RenderMarkdown(Packet{TaskID: "task", Sections: []Section{{
		Key:   "long",
		Title: "Long",
		Order: 10,
		Body:  "abcdef",
	}}}, Options{MaxBytes: 3})
	if !strings.Contains(got, "abc") || !strings.Contains(got, "[truncated: omitted 3 byte(s)]") {
		t.Fatalf("truncate marker missing:\n%s", got)
	}
}

func TestRenderMarkdownAppliesBudgetAcrossSections(t *testing.T) {
	t.Parallel()

	got := RenderMarkdown(Packet{TaskID: "task", Sections: []Section{
		{Key: "first", Title: "First", Order: 10, Body: "abc"},
		{Key: "second", Title: "Second", Order: 20, Body: "def"},
	}}, Options{MaxBytes: 4})
	if !strings.Contains(got, "abc") || !strings.Contains(got, "d") {
		t.Fatalf("budgeted body text missing:\n%s", got)
	}
	if strings.Contains(got, "ef") {
		t.Fatalf("budget was applied per section, not globally:\n%s", got)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("budget exhaustion marker missing:\n%s", got)
	}
}
