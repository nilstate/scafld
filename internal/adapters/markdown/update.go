package markdown

import (
	"fmt"
	"strings"

	"github.com/nilstate/scafld/v2/internal/core/spec"
)

// updateSpecMarkdown applies only the sections whose normalized model changed.
// Parser-owned fields can move forward without re-rendering human-owned prose
// that the normalized model intentionally does not understand.
func updateSpecMarkdown(current []byte, previous spec.Model, next spec.Model) ([]byte, error) {
	currentDoc, err := splitDocument(string(current))
	if err != nil {
		return nil, err
	}
	previousDoc, err := splitDocument(string(Render(previous)))
	if err != nil {
		return nil, err
	}
	nextDoc, err := splitDocument(string(Render(next)))
	if err != nil {
		return nil, err
	}
	replacements := map[string]string{}
	for key, nextSegment := range nextDoc.segments {
		if previousDoc.segments[key] != nextSegment {
			replacements[key] = nextSegment
		}
	}
	if len(replacements) == 0 {
		return current, nil
	}
	var b strings.Builder
	seen := map[string]bool{}
	for _, key := range currentDoc.order {
		seen[key] = true
		if replacement, ok := replacements[key]; ok {
			b.WriteString(replacement)
			continue
		}
		b.WriteString(currentDoc.segments[key])
	}
	for _, key := range nextDoc.order {
		if seen[key] {
			continue
		}
		replacement, ok := replacements[key]
		if !ok {
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
			b.WriteString("\n")
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n\n") {
			b.WriteString("\n")
		}
		b.WriteString(replacement)
	}
	return []byte(b.String()), nil
}

type documentSections struct {
	order    []string
	segments map[string]string
}

type lineSpan struct {
	start int
	end   int
	text  string
}

func splitDocument(text string) (documentSections, error) {
	lines := spanLines(text)
	if len(lines) < 3 || lines[0].text != "---" {
		return documentSections{}, fmt.Errorf("%w: front matter is required", ErrMalformedMarkdown)
	}
	end, err := frontMatterEnd(lineTexts(lines))
	if err != nil {
		return documentSections{}, err
	}
	doc := documentSections{segments: map[string]string{}}
	addSegment := func(key string, start int, end int) error {
		if start >= end {
			return nil
		}
		if _, exists := doc.segments[key]; exists {
			if isManagedSectionKey(key) {
				return fmt.Errorf("%w: duplicate section %s", ErrMalformedMarkdown, key)
			}
			for index := 2; ; index++ {
				candidate := fmt.Sprintf("%s#%d", key, index)
				if _, exists := doc.segments[candidate]; !exists {
					key = candidate
					break
				}
			}
		}
		doc.order = append(doc.order, key)
		doc.segments[key] = text[start:end]
		return nil
	}
	if err := addSegment("__front_matter__", 0, lines[end].end); err != nil {
		return documentSections{}, err
	}
	bodyStart := len(text)
	if end+1 < len(lines) {
		bodyStart = lines[end+1].start
	}
	var starts []struct {
		key   string
		start int
	}
	inFence := false
	for i := end + 1; i < len(lines); i++ {
		line := lines[i].text
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "# ") {
			starts = append(starts, struct {
				key   string
				start int
			}{key: "__title__", start: lines[i].start})
			continue
		}
		if strings.HasPrefix(line, "## ") {
			key := sectionKey(line)
			starts = append(starts, struct {
				key   string
				start int
			}{key: key, start: lines[i].start})
		}
	}
	if inFence {
		return documentSections{}, fmt.Errorf("%w: unclosed code fence", ErrMalformedMarkdown)
	}
	if len(starts) == 0 {
		if bodyStart < len(text) {
			if err := addSegment("__body__", bodyStart, len(text)); err != nil {
				return documentSections{}, err
			}
		}
		return doc, nil
	}
	if bodyStart < starts[0].start {
		if err := addSegment("__preamble__", bodyStart, starts[0].start); err != nil {
			return documentSections{}, err
		}
	}
	for i, start := range starts {
		endOffset := len(text)
		if i+1 < len(starts) {
			endOffset = starts[i+1].start
		}
		if err := addSegment(start.key, start.start, endOffset); err != nil {
			return documentSections{}, err
		}
	}
	return doc, nil
}

func sectionKey(line string) string {
	if match := phaseHeadingPattern.FindStringSubmatch(line); match != nil {
		return "phase:" + fmt.Sprintf("phase%s", match[1])
	}
	return "section:" + strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
}

func isManagedSectionKey(key string) bool {
	if key == "__front_matter__" || key == "__title__" || strings.HasPrefix(key, "phase:") {
		return true
	}
	switch key {
	case "section:current state",
		"section:summary",
		"section:context",
		"section:objectives",
		"section:scope",
		"section:dependencies",
		"section:assumptions",
		"section:touchpoints",
		"section:risks",
		"section:acceptance",
		"section:rollback",
		"section:review",
		"section:self eval",
		"section:deviations",
		"section:metadata",
		"section:origin",
		"section:harden rounds",
		"section:planning log":
		return true
	default:
		return false
	}
}

func spanLines(text string) []lineSpan {
	if text == "" {
		return nil
	}
	var lines []lineSpan
	start := 0
	for start < len(text) {
		next := strings.IndexByte(text[start:], '\n')
		if next == -1 {
			lines = append(lines, lineSpan{start: start, end: len(text), text: text[start:]})
			break
		}
		end := start + next + 1
		lines = append(lines, lineSpan{start: start, end: end, text: text[start : end-1]})
		start = end
	}
	return lines
}

func lineTexts(lines []lineSpan) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = line.text
	}
	return out
}

func samePhaseShape(a []spec.Phase, b []spec.Phase) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Number != b[i].Number {
			return false
		}
	}
	return true
}
