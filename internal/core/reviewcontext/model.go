// Package reviewcontext models the exact context packet given to review providers.
package reviewcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const (
	defaultMaxBytes         = 16 * 1024
	defaultRequiredMaxBytes = 128 * 1024
)

// ErrRequiredContextTooLarge is returned when required provider context cannot
// fit the configured packet budget.
var ErrRequiredContextTooLarge = errors.New("required context exceeds budget")

// Packet is the deterministic reviewer brief before provider-specific transport.
type Packet struct {
	TaskID   string
	Title    string
	Status   string
	Sections []Section
}

// Section is one ordered part of the reviewer brief.
type Section struct {
	Key      string
	Title    string
	Order    int
	Body     string
	Required bool
	Sources  []Source
}

// Source identifies material used to construct a section.
type Source struct {
	Kind   string
	Path   string
	SHA256 string
	Bytes  int
}

// Options controls packet rendering.
type Options struct {
	MaxBytes         int
	RequiredMaxBytes int
	Title            string
}

// SourceForContent returns stable provenance for source text.
func SourceForContent(kind string, path string, content []byte) Source {
	sum := sha256.Sum256(content)
	return Source{
		Kind:   strings.TrimSpace(kind),
		Path:   strings.TrimSpace(path),
		SHA256: hex.EncodeToString(sum[:]),
		Bytes:  len(content),
	}
}

// SourceMarkdownSection returns the canonical source-contract section for
// agent-facing packets. Required sections must render in full.
func SourceMarkdownSection(key string, title string, order int, path string, content []byte) Section {
	return Section{
		Key:      key,
		Title:    title,
		Order:    order,
		Body:     strings.TrimSpace(string(content)),
		Required: true,
		Sources:  []Source{SourceForContent("file", path, content)},
	}
}

// RenderMarkdown renders a packet into deterministic provider-readable Markdown.
func RenderMarkdown(packet Packet, opts Options) string {
	maxBytes := normalizeMaxBytes(opts.MaxBytes)
	requiredMaxBytes := normalizeRequiredMaxBytes(opts.RequiredMaxBytes)
	return renderMarkdown(packet, opts, normalizeSections(packet.Sections), maxBytes, requiredMaxBytes)
}

// RenderMarkdownStrict renders a provider packet after validating that the full
// required context fits the configured packet budget.
func RenderMarkdownStrict(packet Packet, opts Options) (string, error) {
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	requiredMaxBytes := normalizeRequiredMaxBytes(opts.RequiredMaxBytes)
	sections := normalizeSections(packet.Sections)
	if err := validateRequiredBudget(sections, requiredMaxBytes); err != nil {
		return "", err
	}
	return renderMarkdown(packet, opts, sections, maxBytes, requiredMaxBytes), nil
}

func renderMarkdown(packet Packet, opts Options, sections []Section, maxBytes int, requiredMaxBytes int) string {
	rendered := budgetSections(sections, maxBytes)
	var b strings.Builder
	title := strings.TrimSpace(opts.Title)
	if title == "" {
		title = "Review Context Packet"
	}
	fmt.Fprintf(&b, "# %s\n\n", title)
	fmt.Fprintf(&b, "Task: %s\n", strings.TrimSpace(packet.TaskID))
	if strings.TrimSpace(packet.Title) != "" {
		fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(packet.Title))
	}
	if strings.TrimSpace(packet.Status) != "" {
		fmt.Fprintf(&b, "Status: %s\n", strings.TrimSpace(packet.Status))
	}
	fmt.Fprintf(&b, "\n")
	writeBudgetManifest(&b, rendered, maxBytes, requiredMaxBytes)
	for _, section := range rendered {
		if section.Omitted && section.BodyBytes > 0 {
			continue
		}
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = section.Key
		}
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(section.Sources) > 0 {
			b.WriteString("Sources:\n")
			for _, source := range normalizeSources(section.Sources) {
				fmt.Fprintf(&b, "- %s `%s` sha256=%s bytes=%d\n", emptyDefault(source.Kind, "source"), source.Path, source.SHA256, source.Bytes)
			}
			b.WriteString("\n")
		}
		if section.RenderedBody != "" {
			b.WriteString(section.RenderedBody)
			b.WriteString("\n")
		} else {
			b.WriteString("- none\n")
		}
		if section.OmittedBytes > 0 {
			fmt.Fprintf(&b, "\n[truncated: omitted %d byte(s); see Context Budget Manifest]\n", section.OmittedBytes)
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func normalizeMaxBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return defaultMaxBytes
	}
	return maxBytes
}

func normalizeRequiredMaxBytes(maxBytes int) int {
	if maxBytes <= 0 {
		return defaultRequiredMaxBytes
	}
	return maxBytes
}

func validateRequiredBudget(sections []Section, maxBytes int) error {
	requiredBytes := 0
	requiredSections := []string{}
	for _, section := range sections {
		if !section.Required {
			continue
		}
		bodyBytes := len([]byte(strings.TrimSpace(section.Body)))
		requiredBytes += bodyBytes
		title := strings.TrimSpace(section.Title)
		if title == "" {
			title = section.Key
		}
		requiredSections = append(requiredSections, fmt.Sprintf("%s=%d", title, bodyBytes))
	}
	if requiredBytes <= maxBytes {
		return nil
	}
	if len(requiredSections) == 0 {
		requiredSections = []string{"none"}
	}
	return fmt.Errorf("%w: required section body bytes %d exceed max %d (%s)", ErrRequiredContextTooLarge, requiredBytes, maxBytes, strings.Join(requiredSections, ", "))
}

type renderedSection struct {
	Section
	RenderedBody  string
	BodyBytes     int
	RenderedBytes int
	OmittedBytes  int
	Omitted       bool
}

func budgetSections(sections []Section, maxBytes int) []renderedSection {
	remaining := maxBytes
	rendered := make([]renderedSection, 0, len(sections))
	for _, section := range sections {
		bodyText := strings.TrimSpace(section.Body)
		bodyBytes := len([]byte(bodyText))
		item := renderedSection{Section: section, BodyBytes: bodyBytes}
		if bodyBytes == 0 {
			rendered = append(rendered, item)
			continue
		}
		if section.Required {
			item.RenderedBody = bodyText
			item.RenderedBytes = bodyBytes
			rendered = append(rendered, item)
			continue
		}
		if remaining <= 0 {
			item.Omitted = true
			item.OmittedBytes = bodyBytes
			rendered = append(rendered, item)
			continue
		}
		body, omitted := truncateBytes(bodyText, remaining)
		item.RenderedBody = body
		item.RenderedBytes = len([]byte(body))
		item.OmittedBytes = omitted
		if item.RenderedBytes == 0 && omitted > 0 {
			item.Omitted = true
		}
		remaining -= item.RenderedBytes
		rendered = append(rendered, item)
	}
	return rendered
}

func writeBudgetManifest(b *strings.Builder, sections []renderedSection, maxBytes int, requiredMaxBytes int) {
	renderedBytes := 0
	omittedBytes := 0
	included := []renderedSection{}
	truncated := []renderedSection{}
	omitted := []renderedSection{}
	for _, section := range sections {
		renderedBytes += section.RenderedBytes
		omittedBytes += section.OmittedBytes
		switch {
		case section.Omitted && section.BodyBytes > 0:
			omitted = append(omitted, section)
		case section.OmittedBytes > 0:
			truncated = append(truncated, section)
		default:
			included = append(included, section)
		}
	}
	fmt.Fprintf(b, "## Context Budget Manifest\n\n")
	fmt.Fprintf(b, "Max required section body bytes: %d\n", requiredMaxBytes)
	fmt.Fprintf(b, "Max discretionary section body bytes: %d\n", maxBytes)
	fmt.Fprintf(b, "Rendered section body bytes: %d\n", renderedBytes)
	fmt.Fprintf(b, "Omitted section body bytes: %d\n\n", omittedBytes)
	writeManifestGroup(b, "Included sections", included, false)
	writeManifestGroup(b, "Truncated sections", truncated, true)
	writeManifestGroup(b, "Omitted sections", omitted, true)
}

func writeManifestGroup(b *strings.Builder, title string, sections []renderedSection, includeSources bool) {
	fmt.Fprintf(b, "%s:\n", title)
	if len(sections) == 0 {
		b.WriteString("- none\n\n")
		return
	}
	for _, section := range sections {
		titleText := strings.TrimSpace(section.Title)
		if titleText == "" {
			titleText = section.Key
		}
		fmt.Fprintf(b, "- `%s` (%s): rendered=%d body=%d omitted=%d", section.Key, titleText, section.RenderedBytes, section.BodyBytes, section.OmittedBytes)
		if section.Required {
			b.WriteString(" required=true")
		}
		if includeSources {
			if paths := sourcePaths(section.Sources); len(paths) > 0 {
				fmt.Fprintf(b, " sources=%s", strings.Join(paths, ", "))
			}
			if section.Omitted && section.BodyBytes > 0 {
				b.WriteString(" reason=context budget exhausted")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func sourcePaths(sources []Source) []string {
	paths := make([]string, 0, len(sources))
	for _, source := range normalizeSources(sources) {
		if source.Path != "" {
			paths = append(paths, "`"+source.Path+"`")
		}
	}
	return paths
}

func normalizeSections(sections []Section) []Section {
	out := make([]Section, 0, len(sections))
	seen := map[string]bool{}
	for _, section := range sections {
		key := strings.TrimSpace(section.Key)
		if key == "" {
			key = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(section.Title), " ", "_"))
		}
		if key == "" || seen[key] {
			continue
		}
		section.Key = key
		section.Title = strings.TrimSpace(section.Title)
		section.Body = strings.TrimSpace(section.Body)
		section.Sources = normalizeSources(section.Sources)
		out = append(out, section)
		seen[key] = true
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order == out[j].Order {
			return out[i].Key < out[j].Key
		}
		return out[i].Order < out[j].Order
	})
	return out
}

func normalizeSources(sources []Source) []Source {
	out := make([]Source, 0, len(sources))
	seen := map[string]bool{}
	for _, source := range sources {
		source.Kind = strings.TrimSpace(source.Kind)
		source.Path = strings.TrimSpace(source.Path)
		source.SHA256 = strings.TrimSpace(source.SHA256)
		if source.Path == "" {
			continue
		}
		key := source.Kind + "\x00" + source.Path + "\x00" + source.SHA256
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, source)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func truncateBytes(text string, maxBytes int) (string, int) {
	if len(text) <= maxBytes {
		return text, 0
	}
	cut := maxBytes
	for cut > 0 && (text[cut]&0xc0) == 0x80 {
		cut--
	}
	return strings.TrimSpace(text[:cut]), len(text) - cut
}

func emptyDefault(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
