// Package reviewcontext models the exact context packet given to review providers.
package reviewcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const defaultMaxBytes = 16 * 1024

// Packet is the deterministic reviewer brief before provider-specific transport.
type Packet struct {
	TaskID   string
	Title    string
	Status   string
	Sections []Section
}

// Section is one ordered part of the reviewer brief.
type Section struct {
	Key     string
	Title   string
	Order   int
	Body    string
	Sources []Source
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
	MaxBytes int
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

// RenderMarkdown renders a packet into deterministic provider-readable Markdown.
func RenderMarkdown(packet Packet, opts Options) string {
	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	remainingBodyBytes := maxBytes
	sections := normalizeSections(packet.Sections)
	var b strings.Builder
	fmt.Fprintf(&b, "# Review Context Packet\n\n")
	fmt.Fprintf(&b, "Task: %s\n", strings.TrimSpace(packet.TaskID))
	if strings.TrimSpace(packet.Title) != "" {
		fmt.Fprintf(&b, "Title: %s\n", strings.TrimSpace(packet.Title))
	}
	if strings.TrimSpace(packet.Status) != "" {
		fmt.Fprintf(&b, "Status: %s\n", strings.TrimSpace(packet.Status))
	}
	fmt.Fprintf(&b, "\n")
	for _, section := range sections {
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
		body, omitted := truncateBytes(strings.TrimSpace(section.Body), remainingBodyBytes)
		if body != "" {
			b.WriteString(body)
			b.WriteString("\n")
			remainingBodyBytes -= len([]byte(body))
		} else {
			b.WriteString("- none\n")
		}
		if omitted > 0 {
			fmt.Fprintf(&b, "\n[truncated: omitted %d byte(s)]\n", omitted)
		}
		b.WriteString("\n")
		if remainingBodyBytes <= 0 {
			omittedSections := remainingSectionsBodyBytes(sections, section.Key)
			if omittedSections > 0 {
				fmt.Fprintf(&b, "[context budget exhausted: omitted %d byte(s) from remaining section body text]\n\n", omittedSections)
			}
			break
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func remainingSectionsBodyBytes(sections []Section, currentKey string) int {
	afterCurrent := false
	total := 0
	for _, section := range sections {
		if afterCurrent {
			total += len([]byte(strings.TrimSpace(section.Body)))
			continue
		}
		if section.Key == currentKey {
			afterCurrent = true
		}
	}
	return total
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
