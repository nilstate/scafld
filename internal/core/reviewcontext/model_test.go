package reviewcontext

import (
	"errors"
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
	if !strings.Contains(got, "## Context Budget Manifest") || !strings.Contains(got, "Included sections:") {
		t.Fatalf("budget manifest missing:\n%s", got)
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
	if !strings.Contains(got, "abc") || !strings.Contains(got, "[truncated: omitted 3 byte(s); see Context Budget Manifest]") {
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

func TestRenderMarkdownListsOmittedSectionsWithSources(t *testing.T) {
	t.Parallel()

	got := RenderMarkdown(Packet{TaskID: "task", Sections: []Section{
		{Key: "first", Title: "First", Order: 10, Body: "abc"},
		{Key: "second", Title: "Second", Order: 20, Body: "def", Sources: []Source{SourceForContent("file", "docs/review.md", []byte("def"))}},
	}}, Options{MaxBytes: 3})
	if !strings.Contains(got, "Omitted sections:") || !strings.Contains(got, "`second` (Second): rendered=0 body=3 omitted=3 sources=`docs/review.md` reason=context budget exhausted") {
		t.Fatalf("omission manifest missing:\n%s", got)
	}
	if strings.Contains(got, "## Second") {
		t.Fatalf("omitted section body rendered despite exhausted budget:\n%s", got)
	}
}

func TestRenderMarkdownRequiredSectionDoesNotConsumeBudget(t *testing.T) {
	t.Parallel()

	got := RenderMarkdown(Packet{TaskID: "task", Sections: []Section{
		SourceMarkdownSection("source_spec_markdown", "Source Spec Markdown", 5, "task.md", []byte("# Task\n\nfull contract")),
		{Key: "derived", Title: "Derived", Order: 10, Body: "abc"},
	}}, Options{MaxBytes: 2})
	if !strings.Contains(got, "# Task\n\nfull contract") {
		t.Fatalf("required source section was not rendered:\n%s", got)
	}
	if !strings.Contains(got, "ab") || strings.Contains(got, "abc\n") {
		t.Fatalf("discretionary budget was not applied to derived section:\n%s", got)
	}
	if !strings.Contains(got, "`source_spec_markdown` (Source Spec Markdown):") || !strings.Contains(got, "required=true") {
		t.Fatalf("required section was not identified in manifest:\n%s", got)
	}
}

func TestRenderMarkdownStrictRejectsOversizedRequiredSection(t *testing.T) {
	t.Parallel()

	packet := Packet{TaskID: "task", Sections: []Section{
		SourceMarkdownSection("source_spec_markdown", "Source Spec Markdown", 5, "task.md", []byte("abcdef")),
		{Key: "derived", Title: "Derived", Order: 10, Body: "xy"},
	}}
	if got := RenderMarkdown(packet, Options{MaxBytes: 3}); !strings.Contains(got, "abcdef") {
		t.Fatalf("non-strict rendering should keep human context visible:\n%s", got)
	}
	_, err := RenderMarkdownStrict(packet, Options{MaxBytes: 3, RequiredMaxBytes: 3})
	if !errors.Is(err, ErrRequiredContextTooLarge) {
		t.Fatalf("error = %v, want %v", err, ErrRequiredContextTooLarge)
	}
	for _, want := range []string{"required section body bytes 6 exceed max 3", "Source Spec Markdown=6"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestRenderMarkdownStrictSplitsRequiredAndDiscretionaryBudgets(t *testing.T) {
	t.Parallel()

	packet := Packet{TaskID: "task", Sections: []Section{
		SourceMarkdownSection("source_spec_markdown", "Source Spec Markdown", 5, "task.md", []byte("abcdef")),
		{Key: "derived", Title: "Derived", Order: 10, Body: "xyz"},
	}}
	got, err := RenderMarkdownStrict(packet, Options{MaxBytes: 2, RequiredMaxBytes: 6})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "abcdef") {
		t.Fatalf("required source was not rendered in full:\n%s", got)
	}
	if !strings.Contains(got, "xy") || strings.Contains(got, "xyz\n") {
		t.Fatalf("discretionary section was not independently truncated:\n%s", got)
	}
	if !strings.Contains(got, "Max required section body bytes: 6") || !strings.Contains(got, "Max discretionary section body bytes: 2") {
		t.Fatalf("manifest missing split budgets:\n%s", got)
	}
}
